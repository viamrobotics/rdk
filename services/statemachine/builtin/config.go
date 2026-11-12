// Package builtin implements the state machine service as a statechart driven
// by config: nested states, sub-machines, task outcomes, events, watchers,
// timeouts and retries.
package builtin

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/task"
)

const (
	// TargetHistory re-enters whichever sibling state was active before.
	TargetHistory = "$history"
	// TargetDone exits a sub-machine; the trigger becomes the parent state's outcome.
	TargetDone = "$done"

	defaultCancelTimeout = 5 * time.Second
	defaultPoll          = 500 * time.Millisecond
	defaultHistoryLimit  = 100
)

// Task types a state can run without a module.
const (
	TaskTypeResource  = "resource"
	TaskTypeCall      = "call"
	TaskTypeDoCommand = "do_command"
	TaskTypeWaitFor   = "wait_for"
	TaskTypeNone      = "none"
)

// Config is the attributes of a state machine service.
type Config struct {
	// Initial is the name of the state entered on start and on Reset.
	Initial string `json:"initial"`
	// Context is the machine's starting data.
	Context map[string]interface{} `json:"context,omitempty"`
	// Watchers poll resources while the machine runs and emit events.
	Watchers []WatcherConfig `json:"watchers,omitempty"`
	// On maps outcome and event names to target states for the whole machine.
	On map[string]string `json:"on,omitempty"`
	// States are the top-level states.
	States map[string]*StateConfig `json:"states"`
	// Machines are sub-machines a state can reference by name.
	Machines map[string]*MachineConfig `json:"machines,omitempty"`
	// CancelTimeout bounds how long a preempted task may take to stop. Default 5s.
	CancelTimeout string `json:"cancel_timeout,omitempty"`
	// HistoryLimit is how many transitions are retained. Default 100.
	HistoryLimit int `json:"history_limit,omitempty"`
}

// MachineConfig is a sub-machine: a compound state defined once and
// referenced from several states.
type MachineConfig struct {
	Initial  string                  `json:"initial"`
	States   map[string]*StateConfig `json:"states"`
	On       map[string]string       `json:"on,omitempty"`
	Watchers []WatcherConfig         `json:"watchers,omitempty"`
}

// StateConfig is one state.
type StateConfig struct {
	// Task is what runs while in this state. Omitted for pure waiting states.
	Task *TaskConfig `json:"task,omitempty"`
	// On maps outcome and event names to target states.
	On map[string]string `json:"on,omitempty"`
	// Timeout cancels the task and emits "timeout" after this long.
	Timeout string `json:"timeout,omitempty"`
	// Retry re-runs the task on "error" before taking the error transition.
	Retry *RetryConfig `json:"retry,omitempty"`
	// Initial and States make this a compound state.
	Initial string                  `json:"initial,omitempty"`
	States  map[string]*StateConfig `json:"states,omitempty"`
	// Machine makes this a compound state defined by a sub-machine.
	Machine string `json:"machine,omitempty"`
	// Watchers are active while this state or any descendant is.
	Watchers []WatcherConfig `json:"watchers,omitempty"`
}

// TaskConfig is what a state runs.
type TaskConfig struct {
	// Type is one of resource, call, do_command, wait_for or none.
	Type string `json:"type"`
	// Name is the task resource's name, for type resource.
	Name string `json:"name,omitempty"`
	// Input is merged over the machine's context and the event payload as the
	// task resource's input. Strings of the form "$ctx.key" are resolved.
	Input map[string]interface{} `json:"input,omitempty"`
	// Resource is the resource acted on, for call, do_command and wait_for.
	Resource string `json:"resource,omitempty"`
	// Method is the method called, for call and wait_for.
	Method string `json:"method,omitempty"`
	// Args are the method's arguments: a map decoded into the method's request
	// struct, or a list of positional arguments. Strings of the form
	// "$ctx.key" are replaced by the task input's value for key.
	Args interface{} `json:"args,omitempty"`
	// Command is the DoCommand payload, for do_command.
	Command map[string]interface{} `json:"command,omitempty"`
	// Poll is how often wait_for calls the method. Default 500ms.
	Poll string `json:"poll,omitempty"`
	// Condition is the matcher a wait_for result must satisfy.
	Condition map[string]interface{} `json:"condition,omitempty"`
}

// WatcherConfig polls a resource method and emits an event when its
// condition starts to hold.
type WatcherConfig struct {
	Resource  string                 `json:"resource"`
	Method    string                 `json:"method"`
	Args      interface{}            `json:"args,omitempty"`
	Condition map[string]interface{} `json:"condition"`
	Poll      string                 `json:"poll,omitempty"`
	Event     string                 `json:"event"`
}

// RetryConfig re-runs a task on error.
type RetryConfig struct {
	Max     int    `json:"max"`
	Backoff string `json:"backoff,omitempty"`
}

// Validate validates the config and lists the resources it depends on.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	if conf.Initial == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "initial")
	}
	if len(conf.States) == 0 {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "states")
	}
	if _, err := conf.cancelTimeout(); err != nil {
		return nil, nil, resource.NewConfigValidationError(path, err)
	}
	root, err := compile(conf)
	if err != nil {
		return nil, nil, resource.NewConfigValidationError(path, err)
	}
	deps := map[string]struct{}{}
	root.walk(func(n *node) {
		if n.task != nil {
			switch n.task.Type {
			case TaskTypeResource:
				deps[task.Named(n.task.Name).String()] = struct{}{}
			case TaskTypeCall, TaskTypeDoCommand, TaskTypeWaitFor:
				deps[n.task.Resource] = struct{}{}
			}
		}
		for _, w := range n.watchers {
			deps[w.Resource] = struct{}{}
		}
	})
	out := make([]string, 0, len(deps))
	for d := range deps {
		out = append(out, d)
	}
	return out, nil, nil
}

func (conf *Config) cancelTimeout() (time.Duration, error) {
	return parseDuration(conf.CancelTimeout, defaultCancelTimeout)
}

func (conf *Config) historyLimit() int {
	if conf.HistoryLimit <= 0 {
		return defaultHistoryLimit
	}
	return conf.HistoryLimit
}

func parseDuration(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid duration %q", s)
	}
	return d, nil
}

func (tc *TaskConfig) validate(where string) error {
	switch tc.Type {
	case TaskTypeResource:
		if tc.Name == "" {
			return fmt.Errorf("%s: task of type resource needs a name", where)
		}
	case TaskTypeCall:
		if tc.Resource == "" || tc.Method == "" {
			return fmt.Errorf("%s: task of type call needs a resource and a method", where)
		}
	case TaskTypeDoCommand:
		if tc.Resource == "" {
			return fmt.Errorf("%s: task of type do_command needs a resource", where)
		}
	case TaskTypeWaitFor:
		if tc.Resource == "" || tc.Method == "" {
			return fmt.Errorf("%s: task of type wait_for needs a resource and a method", where)
		}
		if _, err := parseDuration(tc.Poll, defaultPoll); err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
	case TaskTypeNone, "":
	default:
		return fmt.Errorf("%s: unknown task type %q", where, tc.Type)
	}
	return nil
}

func (wc *WatcherConfig) validate(where string) error {
	if wc.Resource == "" || wc.Method == "" || wc.Event == "" {
		return fmt.Errorf("%s: a watcher needs a resource, a method and an event", where)
	}
	if _, err := parseDuration(wc.Poll, defaultPoll); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	return nil
}

// node is one compiled state. The root node is the machine itself.
type node struct {
	name     string
	parent   *node
	children map[string]*node
	initial  string
	on       map[string]string
	task     *TaskConfig
	timeout  time.Duration
	retry    *RetryConfig
	backoff  time.Duration
	watchers []WatcherConfig

	// lastLeaf is the leaf most recently active under this node, for $history.
	lastLeaf *node
}

func (n *node) path() []string {
	if n == nil || n.parent == nil {
		return []string{}
	}
	return append(n.parent.path(), n.name)
}

func (n *node) pathString() string {
	return strings.Join(n.path(), ".")
}

func (n *node) isLeaf() bool {
	return len(n.children) == 0
}

func (n *node) walk(fn func(*node)) {
	fn(n)
	for _, c := range n.children {
		c.walk(fn)
	}
}

// compile turns the config into a tree of nodes, expanding sub-machine
// references, and checks that every transition target resolves.
func compile(conf *Config) (*node, error) {
	root := &node{name: "", on: conf.On, initial: conf.Initial, watchers: conf.Watchers}
	for _, w := range conf.Watchers {
		if err := w.validate("machine watcher"); err != nil {
			return nil, err
		}
	}
	if err := compileChildren(root, conf.States, conf.Machines, map[string]bool{}); err != nil {
		return nil, err
	}
	if _, ok := root.children[conf.Initial]; !ok {
		return nil, fmt.Errorf("initial state %q is not a top-level state", conf.Initial)
	}
	var err error
	root.walk(func(n *node) {
		if err != nil {
			return
		}
		for trigger, target := range n.on {
			if target == TargetHistory || target == TargetDone {
				continue
			}
			if resolveTarget(n, target) == nil {
				err = fmt.Errorf("state %q: transition %q targets unknown state %q", n.pathString(), trigger, target)
				return
			}
		}
	})
	return root, err
}

func compileChildren(parent *node, states map[string]*StateConfig, machines map[string]*MachineConfig, expanding map[string]bool) error {
	parent.children = make(map[string]*node, len(states))
	for name, sc := range states {
		if sc == nil {
			sc = &StateConfig{}
		}
		child, err := compileState(parent, name, sc, machines, expanding)
		if err != nil {
			return err
		}
		parent.children[name] = child
	}
	if len(states) > 0 {
		if parent.initial == "" {
			return fmt.Errorf("compound state %q needs an initial state", parent.pathString())
		}
		if _, ok := parent.children[parent.initial]; !ok {
			return fmt.Errorf("compound state %q: initial state %q does not exist", parent.pathString(), parent.initial)
		}
	}
	return nil
}

func compileState(
	parent *node, name string, sc *StateConfig, machines map[string]*MachineConfig, expanding map[string]bool,
) (*node, error) {
	n := &node{name: name, parent: parent, on: sc.On, task: sc.Task, retry: sc.Retry, watchers: sc.Watchers, initial: sc.Initial}
	where := "state " + n.pathString()
	var err error
	if n.timeout, err = parseDuration(sc.Timeout, 0); err != nil {
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	if sc.Retry != nil {
		if n.backoff, err = parseDuration(sc.Retry.Backoff, 0); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
	}
	if sc.Task != nil {
		if err := sc.Task.validate(where); err != nil {
			return nil, err
		}
		if sc.Task.Type == TaskTypeNone || sc.Task.Type == "" {
			n.task = nil
		}
	}
	for _, w := range sc.Watchers {
		if err := w.validate(where); err != nil {
			return nil, err
		}
	}
	states := sc.States
	if sc.Machine != "" {
		if len(sc.States) > 0 {
			return nil, fmt.Errorf("%s: machine and states are mutually exclusive", where)
		}
		if n.task != nil {
			return nil, fmt.Errorf("%s: a state that references a machine cannot run a task", where)
		}
		mc, ok := machines[sc.Machine]
		if !ok || mc == nil {
			return nil, fmt.Errorf("%s: unknown machine %q", where, sc.Machine)
		}
		if expanding[sc.Machine] {
			return nil, fmt.Errorf("%s: machine %q references itself", where, sc.Machine)
		}
		expanding[sc.Machine] = true
		defer delete(expanding, sc.Machine)
		states = mc.States
		n.initial = mc.Initial
		n.on = mergeOn(mc.On, sc.On)
		n.watchers = append(append([]WatcherConfig{}, mc.Watchers...), sc.Watchers...)
		for _, w := range mc.Watchers {
			if err := w.validate("machine " + sc.Machine); err != nil {
				return nil, err
			}
		}
	}
	if len(states) > 0 {
		if n.task != nil {
			return nil, fmt.Errorf("%s: a compound state cannot run a task", where)
		}
		if err := compileChildren(n, states, machines, expanding); err != nil {
			return nil, err
		}
	}
	return n, nil
}

// mergeOn overlays override on base; override wins.
func mergeOn(base, override map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

// resolveTarget finds the state a transition on handler targets: handler's
// siblings first, then each ancestor's siblings in turn.
func resolveTarget(handler *node, target string) *node {
	for scope := handler.parent; scope != nil; scope = scope.parent {
		if t, ok := scope.children[target]; ok {
			return t
		}
	}
	if handler.parent == nil {
		if t, ok := handler.children[target]; ok {
			return t
		}
	}
	return nil
}
