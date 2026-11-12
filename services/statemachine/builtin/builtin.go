package builtin

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/statemachine"
	"go.viam.com/rdk/services/task"
)

func init() {
	resource.RegisterService(statemachine.API, resource.DefaultServiceModel, resource.Registration[statemachine.Service, *Config]{
		Constructor: NewBuiltIn,
	})
}

type cmdKind int

const (
	cmdEvent cmdKind = iota
	cmdStop
	cmdReset
)

type command struct {
	kind    cmdKind
	event   string
	payload map[string]interface{}
	reply   chan commandResult
}

type commandResult struct {
	handled bool
	path    []string
}

// execution is one run of a state's task.
type execution struct {
	node    *node
	svc     task.Service
	attempt int
	cancel  context.CancelFunc
	// done is closed once the task has returned, before taskDone is sent.
	done chan struct{}
}

// progress asks a running task resource for its latest progress. Built-in
// tasks report none.
func (e *execution) progress(ctx context.Context) *task.Progress {
	if e == nil || e.svc == nil {
		return nil
	}
	st, err := e.svc.GetStatus(ctx, nil)
	if err != nil || st.Phase != task.PhaseRunning {
		return nil
	}
	return st.Progress
}

type taskDone struct {
	exec     *execution
	res      task.Result
	err      error
	timedOut bool
}

type watcherEvent struct {
	event   string
	payload map[string]interface{}
}

// builtIn drives a compiled statechart. One goroutine (loop) owns the
// machine's state; API calls hand it commands and read a published snapshot.
type builtIn struct {
	resource.Named
	logger        logging.Logger
	conf          *Config
	definition    map[string]interface{}
	root          *node
	runners       map[*node]*runner
	watchers      map[*node][]*watcher
	cancelTimeout time.Duration
	historyLimit  int

	cmds     chan command
	taskDone chan taskDone
	events   chan watcherEvent

	// Owned by loop.
	status         statemachine.Status
	active         *node
	enteredAt      time.Time
	context        map[string]interface{}
	exec           *execution
	activeWatchers map[*node]context.CancelFunc

	// Published for readers.
	snapMu   sync.RWMutex
	snap     statemachine.State
	snapExec *execution
	history  []statemachine.Transition

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewBuiltIn constructs a state machine from its config and enters the
// initial state.
func NewBuiltIn(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (statemachine.Service, error) {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	root, err := compile(cfg)
	if err != nil {
		return nil, err
	}
	cancelTimeout, err := cfg.cancelTimeout()
	if err != nil {
		return nil, err
	}

	b := &builtIn{
		Named:          conf.ResourceName().AsNamed(),
		logger:         logger,
		conf:           cfg,
		definition:     map[string]interface{}(conf.Attributes),
		root:           root,
		runners:        map[*node]*runner{},
		watchers:       map[*node][]*watcher{},
		cancelTimeout:  cancelTimeout,
		historyLimit:   cfg.historyLimit(),
		cmds:           make(chan command),
		taskDone:       make(chan taskDone, 16),
		events:         make(chan watcherEvent, 16),
		status:         statemachine.StatusRunning,
		context:        copyMap(cfg.Context),
		activeWatchers: map[*node]context.CancelFunc{},
	}
	root.walk(func(n *node) {
		if err != nil {
			return
		}
		if n.task != nil {
			var r *runner
			if r, err = buildRunner(n.task, deps); err != nil {
				return
			}
			b.runners[n] = r
		}
		for _, wc := range n.watchers {
			var w *watcher
			if w, err = buildWatcher(wc, deps); err != nil {
				return
			}
			b.watchers[n] = append(b.watchers[n], w)
		}
	})
	if err != nil {
		return nil, err
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.enter(root.children[cfg.Initial], nil, statemachine.TriggerInitial, nil, nil)
	b.wg.Add(1)
	go b.loop()
	return b, nil
}

func (b *builtIn) loop() {
	defer b.wg.Done()
	for {
		select {
		case <-b.ctx.Done():
			return
		case c := <-b.cmds:
			c.reply <- b.handleCommand(c)
		case d := <-b.taskDone:
			b.handleTaskDone(d)
		case e := <-b.events:
			b.handleEvent(e.event, e.payload)
		}
	}
}

func (b *builtIn) handleCommand(c command) commandResult {
	switch c.kind {
	case cmdEvent:
		handled := b.handleEvent(c.event, c.payload)
		return commandResult{handled: handled, path: b.active.path()}
	case cmdStop:
		b.stop()
	case cmdReset:
		b.reset()
	}
	return commandResult{path: b.active.path()}
}

// handleEvent looks an event up from the active state outward and takes the
// first transition that matches. An unmatched event is dropped.
func (b *builtIn) handleEvent(event string, payload map[string]interface{}) bool {
	handler, target, ok := lookup(b.active, event)
	if !ok {
		b.logger.Debugw("event has no transition in the active state or its ancestors; dropped", "event", event, "state", b.active.pathString())
		return false
	}
	b.logger.Infow("event", "event", event, "state", b.active.pathString(), "target", target)
	b.takeTransition(handler, target, event, payload, nil)
	return true
}

func (b *builtIn) handleTaskDone(d taskDone) {
	if d.exec != b.exec {
		return
	}
	b.exec = nil
	n := d.exec.node
	var result *task.Result
	switch {
	case d.timedOut:
		result = &task.Result{Outcome: task.OutcomeTimeout}
	case d.err != nil:
		if b.retry(n, d.exec.attempt, d.err.Error()) {
			return
		}
		result = &task.Result{Outcome: task.OutcomeError, Output: map[string]interface{}{"error": d.err.Error()}}
	default:
		res := d.res
		if res.Outcome == "" {
			res.Outcome = task.OutcomeDone
		}
		if res.Outcome == task.OutcomeError && b.retry(n, d.exec.attempt, "task returned outcome error") {
			return
		}
		if res.Outcome != task.OutcomeError {
			for k, v := range res.Output {
				b.context[k] = v
			}
		}
		result = &res
	}
	b.logger.Infow("task finished", "state", n.pathString(), "outcome", result.Outcome)
	b.handleOutcome(n, result)
}

// retry re-runs the state's task if it has attempts left.
func (b *builtIn) retry(n *node, attempt int, reason string) bool {
	if n.retry == nil || attempt >= n.retry.Max {
		return false
	}
	b.logger.Warnw("task failed; retrying", "state", n.pathString(), "attempt", attempt+1, "max", n.retry.Max, "reason", reason)
	b.startTask(n, nil, attempt+1)
	return true
}

// handleOutcome looks the outcome up from the state outward. An "error" with
// no handler faults the machine; any other unhandled outcome leaves the
// machine waiting in its state.
func (b *builtIn) handleOutcome(from *node, result *task.Result) {
	handler, target, ok := lookup(from, result.Outcome)
	if !ok {
		if result.Outcome == task.OutcomeError {
			b.fault("task in state " + from.pathString() + " returned error with no handler")
			return
		}
		b.logger.Infow("outcome has no transition; waiting", "state", from.pathString(), "outcome", result.Outcome)
		b.publish()
		return
	}
	b.takeTransition(handler, target, result.Outcome, nil, result)
}

// takeTransition resolves target relative to the handler that declared it
// and enters it. $done re-raises the trigger as the enclosing compound
// state's outcome; $history re-enters the handler's previously active sibling.
func (b *builtIn) takeTransition(handler *node, target, trigger string, payload map[string]interface{}, result *task.Result) {
	for {
		switch target {
		case TargetDone:
			exit := handler
			if handler.isLeaf() {
				exit = handler.parent
			}
			if exit == nil || exit.parent == nil {
				b.logger.Errorw("$done used outside a sub-machine; ignored", "state", handler.pathString(), "trigger", trigger)
				return
			}
			var ok bool
			start := exit
			if exit == handler {
				start = exit.parent
			}
			if handler, target, ok = lookup(start, trigger); !ok {
				b.logger.Infow("sub-machine finished; no transition for its outcome; waiting", "state", exit.pathString(), "outcome", trigger)
				b.publish()
				return
			}
		case TargetHistory:
			scope := handler.parent
			if scope == nil {
				scope = handler
			}
			prev := scope.lastLeaf
			if prev == nil || isUnder(prev, handler) && handler != scope {
				b.enter(scope.children[scope.initial], nil, trigger, payload, result)
				return
			}
			b.enter(childContaining(scope, prev), prev, trigger, payload, result)
			return
		default:
			b.enter(resolveTarget(handler, target), nil, trigger, payload, result)
			return
		}
	}
}

// enter leaves the active state, records the transition and enters target,
// descending through initial states (or to prevLeaf, for history) to a leaf.
func (b *builtIn) enter(target, prevLeaf *node, trigger string, payload map[string]interface{}, result *task.Result) {
	if !b.cancelCurrent() {
		return
	}
	newLeaf := target
	if prevLeaf != nil && isUnder(prevLeaf, target) {
		newLeaf = prevLeaf
	}
	for !newLeaf.isLeaf() {
		newLeaf = newLeaf.children[newLeaf.initial]
	}

	old := b.active
	oldPath := []string{}
	if old != nil {
		oldPath = old.path()
		for a := old.parent; a != nil; a = a.parent {
			if childContaining(a, old) != childContaining(a, newLeaf) {
				a.lastLeaf = old
			}
		}
	}
	b.record(statemachine.Transition{From: oldPath, To: newLeaf.path(), Trigger: trigger, Timestamp: time.Now(), TaskResult: result})
	if old != nil {
		b.logger.Infow("transition", "from", old.pathString(), "to", newLeaf.pathString(), "trigger", trigger)
	}

	for n, cancel := range b.activeWatchers {
		if !isUnder(newLeaf, n) {
			cancel()
			delete(b.activeWatchers, n)
		}
	}
	b.active = newLeaf
	b.enteredAt = time.Now()
	b.status = statemachine.StatusRunning
	for n := newLeaf; n != nil; n = n.parent {
		if _, running := b.activeWatchers[n]; !running && len(b.watchers[n]) > 0 {
			b.startWatchers(n)
		}
	}
	b.startTask(newLeaf, payload, 0)
}

func (b *builtIn) startWatchers(n *node) {
	ctx, cancel := context.WithCancel(b.ctx)
	b.activeWatchers[n] = cancel
	for _, w := range b.watchers[n] {
		b.wg.Add(1)
		go func(w *watcher) {
			defer b.wg.Done()
			w.run(ctx, b.logger, func(event string, payload map[string]interface{}) {
				select {
				case b.events <- watcherEvent{event: event, payload: payload}:
				case <-ctx.Done():
				}
			})
		}(w)
	}
}

// startTask runs the leaf's task, if it has one, with the machine's context
// merged with the event payload as input.
func (b *builtIn) startTask(leaf *node, payload map[string]interface{}, attempt int) {
	b.exec = nil
	run, ok := b.runners[leaf]
	if !ok {
		b.publish()
		return
	}
	input := copyMap(b.context)
	for k, v := range payload {
		input[k] = v
	}
	ctx, cancel := context.WithCancel(b.ctx)
	if leaf.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, leaf.timeout)
	}
	exec := &execution{node: leaf, svc: run.svc, attempt: attempt, cancel: cancel, done: make(chan struct{})}
	b.exec = exec
	b.publish()

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer cancel()
		d := taskDone{exec: exec}
		if attempt > 0 && leaf.backoff > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(leaf.backoff):
			}
		}
		if ctx.Err() == nil {
			d.res, d.err = run.run(ctx, input)
		} else {
			d.err = ctx.Err()
		}
		d.timedOut = d.err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded)
		close(exec.done)
		select {
		case b.taskDone <- d:
		case <-b.ctx.Done():
		}
	}()
}

// cancelCurrent cancels the running task and waits for it to stop, bounded
// by the cancel timeout. A task that will not yield in time faults the machine.
func (b *builtIn) cancelCurrent() bool {
	exec := b.exec
	if exec == nil {
		return true
	}
	b.exec = nil
	exec.cancel()
	select {
	case <-exec.done:
		return true
	case <-time.After(b.cancelTimeout):
		b.fault("task in state " + exec.node.pathString() + " did not stop within " + b.cancelTimeout.String())
		return false
	}
}

func (b *builtIn) stop() {
	b.logger.Infow("stop", "state", b.active.pathString())
	if !b.cancelCurrent() {
		return
	}
	b.status = statemachine.StatusStopped
	path := b.active.path()
	b.record(statemachine.Transition{From: path, To: path, Trigger: statemachine.TriggerStop, Timestamp: time.Now()})
	b.publish()
}

func (b *builtIn) reset() {
	b.logger.Infow("reset", "state", b.active.pathString())
	b.context = copyMap(b.conf.Context)
	b.root.walk(func(n *node) { n.lastLeaf = nil })
	b.enter(b.root.children[b.conf.Initial], nil, statemachine.TriggerReset, nil, nil)
}

func (b *builtIn) fault(reason string) {
	b.logger.Errorw("state machine faulted", "state", b.active.pathString(), "reason", reason)
	b.exec = nil
	b.status = statemachine.StatusFaulted
	path := b.active.path()
	b.record(statemachine.Transition{
		From: path, To: path, Trigger: statemachine.TriggerFault, Timestamp: time.Now(),
		TaskResult: &task.Result{Outcome: task.OutcomeError, Output: map[string]interface{}{"error": reason}},
	})
	b.publish()
}

func (b *builtIn) record(tr statemachine.Transition) {
	b.snapMu.Lock()
	defer b.snapMu.Unlock()
	b.history = append(b.history, tr)
	if len(b.history) > b.historyLimit {
		b.history = b.history[len(b.history)-b.historyLimit:]
	}
}

func (b *builtIn) publish() {
	b.snapMu.Lock()
	defer b.snapMu.Unlock()
	b.snap = statemachine.State{Path: b.active.path(), Status: b.status, Context: copyMap(b.context), EnteredAt: b.enteredAt}
	b.snapExec = b.exec
}

func (b *builtIn) send(ctx context.Context, c command) (commandResult, error) {
	c.reply = make(chan commandResult, 1)
	select {
	case b.cmds <- c:
	case <-ctx.Done():
		return commandResult{}, ctx.Err()
	case <-b.ctx.Done():
		return commandResult{}, errors.New("state machine is closed")
	}
	select {
	case r := <-c.reply:
		return r, nil
	case <-ctx.Done():
		return commandResult{}, ctx.Err()
	case <-b.ctx.Done():
		return commandResult{}, errors.New("state machine is closed")
	}
}

func (b *builtIn) GetState(ctx context.Context, extra map[string]interface{}) (statemachine.State, error) {
	b.snapMu.RLock()
	st := b.snap
	exec := b.snapExec
	b.snapMu.RUnlock()
	st.Path = append([]string{}, st.Path...)
	st.Context = copyMap(st.Context)
	st.TaskProgress = exec.progress(ctx)
	return st, nil
}

func (b *builtIn) SendEvent(ctx context.Context, event string, payload, extra map[string]interface{}) (statemachine.EventResult, error) {
	r, err := b.send(ctx, command{kind: cmdEvent, event: event, payload: payload})
	if err != nil {
		return statemachine.EventResult{}, err
	}
	return statemachine.EventResult{Handled: r.handled, Path: r.path}, nil
}

func (b *builtIn) GetHistory(ctx context.Context, limit int, extra map[string]interface{}) ([]statemachine.Transition, error) {
	b.snapMu.RLock()
	defer b.snapMu.RUnlock()
	h := b.history
	if limit > 0 && limit < len(h) {
		h = h[len(h)-limit:]
	}
	return append([]statemachine.Transition{}, h...), nil
}

func (b *builtIn) GetDefinition(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return copyMap(b.definition), nil
}

func (b *builtIn) Stop(ctx context.Context, extra map[string]interface{}) error {
	_, err := b.send(ctx, command{kind: cmdStop})
	return err
}

func (b *builtIn) Reset(ctx context.Context, extra map[string]interface{}) error {
	_, err := b.send(ctx, command{kind: cmdReset})
	return err
}

// Status reports the active path and status, for the generic GetStatus.
func (b *builtIn) Status(ctx context.Context) (map[string]interface{}, error) {
	st, err := b.GetState(ctx, nil)
	if err != nil {
		return nil, err
	}
	path := make([]interface{}, len(st.Path))
	for i, p := range st.Path {
		path[i] = p
	}
	return map[string]interface{}{"path": path, "status": st.Status.String()}, nil
}

// Close cancels the running task and every watcher.
func (b *builtIn) Close(ctx context.Context) error {
	b.cancel()
	b.wg.Wait()
	return nil
}

// lookup finds the first transition for trigger in from's on map or any ancestor's.
func lookup(from *node, trigger string) (handler *node, target string, ok bool) {
	for n := from; n != nil; n = n.parent {
		if t, found := n.on[trigger]; found {
			return n, t, true
		}
	}
	return nil, "", false
}

// isUnder reports whether n is ancestor or self of... that is, whether leaf is
// ancestor itself or one of its descendants.
func isUnder(leaf, ancestor *node) bool {
	for n := leaf; n != nil; n = n.parent {
		if n == ancestor {
			return true
		}
	}
	return false
}

// childContaining returns the child of parent that is or contains n.
func childContaining(parent, n *node) *node {
	for c := n; c != nil; c = c.parent {
		if c.parent == parent {
			return c
		}
	}
	return nil
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
