package builtin

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// jointPositionsChItem is one live joint-space waypoint fed to the streaming executor.
type jointPositionsChItem struct {
	// Positions are the target joint positions for this waypoint.
	Positions []referenceframe.Input

	// StopAcceptable ends the current trajex session and arm stream at this point and
	// starts a fresh one. Use it at segment boundaries where the arm may briefly
	// stop (e.g. between sanding strokes).
	StopAcceptable bool
}

// stream manages a single arm-streaming session: a goroutine runs the
// executor (run) while DoCommand calls push joint-space targets onto jointPositionsCh
// across many requests.
//
// The session outlives individual DoCommand requests, so its goroutine runs on
// a background context (cancelled via cancel) rather than a request context.
type stream struct {
	logger  logging.Logger
	armName string
	dof     int

	jointPositionsCh chan jointPositionsChItem

	cancel context.CancelFunc
	done   chan struct{} // closed when StreamJointTargets returns

	// targetsClosed guards against double-closing jointPositionsCh. Only written while
	// holding builtIn.streamMu for writing.
	targetsClosed bool

	// resultErr is set exactly once, before done is closed, so it is safe to read
	// in any branch guarded by a receive on done.
	resultErr error

	// trace records producer/consumer buffer occupancy, call timings, and PVAT velocities for
	// this session, for diagnosing pacing/buffering issues. Exposed via stream_status.
	trace *pipelineTrace
}

// streamStart resolves the named arm, reads a seed configuration if one
// was not supplied, and launches the streaming executor on a background goroutine.
func (ms *builtIn) streamStart(
	ctx context.Context,
	armName string,
	cfg streamConfig,
	seed []referenceframe.Input,
) error {
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	ms.streamMu.Lock()
	defer ms.streamMu.Unlock()

	if ms.stream != nil {
		select {
		case <-ms.stream.done:
			// A prior session already finished; replace it.
			ms.stream = nil
		default:
			return fmt.Errorf("a stream is already running; call %s or %s first", DoStreamFlush, DoStreamAbort)
		}
	}

	ms.mu.RLock()
	r, ok := ms.components[armName]
	ms.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no component named %q is known to the motion service", armName)
	}
	a, err := utils.AssertType[arm.Arm](r)
	if err != nil {
		return fmt.Errorf("component %q is not an arm: %w", armName, err)
	}

	if seed == nil {
		seed, err = a.JointPositions(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to read seed joint positions from %q: %w", armName, err)
		}
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	s := &stream{
		logger:           ms.logger.Sublogger("arm_streaming"),
		armName:          armName,
		dof:              len(seed),
		jointPositionsCh: make(chan jointPositionsChItem),
		cancel:           cancel,
		done:             make(chan struct{}),
		trace:            newPipelineTrace(),
	}

	go func() {
		err := s.run(streamCtx, a, seed, cfg)
		s.resultErr = err
		if err != nil {
			s.logger.CWarnf(streamCtx, "arm streaming session ended with error: %v", err)
		}
		close(s.done)
	}()

	ms.stream = s
	return nil
}

func (ms *builtIn) streamPush(ctx context.Context, jpChItem []jointPositionsChItem) (int, error) {
	ms.streamMu.RLock()
	defer ms.streamMu.RUnlock()

	s := ms.stream
	if s == nil {
		return 0, fmt.Errorf("no streaming session is running; call %s first", DoStreamStart)
	}

	sent := 0
	for _, jpItem := range jpChItem {
		if len(jpItem.Positions) != s.dof {
			return sent, fmt.Errorf("target has %d joint positions, but the arm has %d joints", len(jpItem.Positions), s.dof)
		}
		if err := s.send(ctx, jpItem); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

// streamFlush ends the active session gracefully: the joint-positions channel is
// closed so the executor drains the remaining trajectory to the arm before stopping.
func (ms *builtIn) streamFlush(ctx context.Context) map[string]any {
	return ms.streamEnd(ctx, false)
}

// streamAbort ends the active session immediately: the session context is
// cancelled, so any buffered trajectory that hasn't reached the arm is dropped.
func (ms *builtIn) streamAbort(ctx context.Context) map[string]any {
	return ms.streamEnd(ctx, true)
}

// streamEnd ends the active session. When abort is false the targets
// channel is closed so the executor drains the remaining trajectory to the arm;
// when true the session context is cancelled to stop immediately.
func (ms *builtIn) streamEnd(ctx context.Context, abort bool) map[string]any {
	ms.streamMu.RLock()
	s := ms.stream
	ms.streamMu.RUnlock()
	if s == nil {
		return map[string]any{"running": false}
	}

	// Abort unblocks any in-flight push via done (StreamJointTargets returns on
	// cancel). Graceful stop relies on the write lock below to serialize with
	// pushes, which take the read lock while sending.
	if abort {
		s.cancel()
	}

	ms.streamMu.Lock()
	if ms.stream == s {
		if !abort && !s.targetsClosed {
			close(s.jointPositionsCh)
			s.targetsClosed = true
		}
		ms.stream = nil
	}
	ms.streamMu.Unlock()

	select {
	case <-s.done:
	case <-ctx.Done():
		// Caller gave up waiting; make sure the session still tears down.
		s.cancel()
		<-s.done
	}

	status := map[string]any{"running": false, "trace": s.trace.snapshot()}
	if s.resultErr != nil {
		status["error"] = s.resultErr.Error()
	}
	return status
}

func (ms *builtIn) streamStatus() map[string]any {
	ms.streamMu.RLock()
	s := ms.stream
	ms.streamMu.RUnlock()
	if s == nil {
		return map[string]any{"running": false}
	}

	finished := false
	select {
	case <-s.done:
		finished = true
	default:
	}
	status := map[string]any{
		"running": !finished,
		"arm":     s.armName,
		"trace":   s.trace.snapshot(),
	}
	if finished && s.resultErr != nil {
		status["error"] = s.resultErr.Error()
	}
	return status
}

// handleStreamCommand handles arm-streaming DoCommand requests. It returns
// (response, handled, error); when handled is false the caller should continue
// processing other DoCommand keys.
func (ms *builtIn) handleStreamCommand(
	ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, bool, error) {
	if req, ok := cmd[DoStreamStart]; ok {
		armName, cfg, seed, err := parseStreamStart(req)
		if err != nil {
			return nil, true, err
		}
		if err := ms.streamStart(ctx, armName, cfg, seed); err != nil {
			return nil, true, err
		}
		return map[string]interface{}{DoStreamStart: true}, true, nil
	}

	if req, ok := cmd[DoStreamPush]; ok {
		targets, err := parseStreamTargets(req)
		if err != nil {
			return nil, true, err
		}
		sent, err := ms.streamPush(ctx, targets)
		if err != nil {
			return nil, true, err
		}
		return map[string]interface{}{DoStreamPush: sent}, true, nil
	}

	if _, ok := cmd[DoStreamFlush]; ok {
		return map[string]interface{}{DoStreamFlush: ms.streamFlush(ctx)}, true, nil
	}

	if _, ok := cmd[DoStreamAbort]; ok {
		return map[string]interface{}{DoStreamAbort: ms.streamAbort(ctx)}, true, nil
	}

	if _, ok := cmd[DoStreamStatus]; ok {
		return map[string]interface{}{DoStreamStatus: ms.streamStatus()}, true, nil
	}

	return nil, false, nil
}

func parseStreamStart(req interface{}) (string, streamConfig, []referenceframe.Input, error) {
	var cfg streamConfig
	m, err := utils.AssertType[map[string]interface{}](req)
	if err != nil {
		// Also accept a bare arm-name string for the common default-config case.
		if s, ok := req.(string); ok {
			m = map[string]interface{}{"arm": s}
		} else {
			return "", cfg, nil, fmt.Errorf("%s expects an object or an arm-name string", DoStreamStart)
		}
	}

	armName, _ := m["arm"].(string)
	if armName == "" {
		armName, _ = m["component_name"].(string)
	}
	if armName == "" {
		return "", cfg, nil, fmt.Errorf(`%s requires an "arm" (or "component_name") field`, DoStreamStart)
	}

	if rawCfg, ok := m["config"]; ok {
		if err := parseStreamConfig(rawCfg, &cfg); err != nil {
			return "", cfg, nil, fmt.Errorf("invalid streaming config: %w", err)
		}
	}

	var seed []referenceframe.Input
	if rawSeed, ok := m["seed"]; ok {
		seed, err = toInputs(rawSeed)
		if err != nil {
			return "", cfg, nil, fmt.Errorf("invalid seed: %w", err)
		}
	}
	return armName, cfg, seed, nil
}

// parseStreamTargets accepts either a single joint-position vector ([j0, j1, ...])
// or a list of vectors ([[...], [...]]).
func parseStreamTargets(req interface{}) ([]jointPositionsChItem, error) {
	arr, ok := req.([]interface{})
	if !ok || len(arr) == 0 {
		return nil, fmt.Errorf("%s expects a non-empty list of joint positions", DoStreamPush)
	}

	var vectors [][]referenceframe.Input
	if _, is2D := arr[0].([]interface{}); is2D {
		for i, e := range arr {
			vec, err := toInputs(e)
			if err != nil {
				return nil, fmt.Errorf("target %d: %w", i, err)
			}
			vectors = append(vectors, vec)
		}
	} else {
		vec, err := toInputs(req)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, vec)
	}

	targets := make([]jointPositionsChItem, len(vectors))
	for i, vec := range vectors {
		targets[i] = jointPositionsChItem{Positions: vec}
	}
	return targets, nil
}

// send delivers one target onto the session channel. Callers must hold
// builtIn.streamMu for reading so that a graceful stop (which closes jointPositionsCh
// while holding the write lock) can never race with an in-flight send.
func (s *stream) send(ctx context.Context, t jointPositionsChItem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return fmt.Errorf("streaming session ended: %w", s.resultErr)
	case s.jointPositionsCh <- t:
		return nil
	}
}

// run executes the streaming session: it smooths the joint-space targets pushed
// onto jointPositionsCh through a trajex session and flow-controls a bidirectional
// stream to the arm, seeding the first trajex session at seed.
//
// Return contract:
//   - nil once jointPositionsCh is closed and every sampled PVAT has been drained to
//     the arm and acknowledged;
//   - the first non-context error from either the producer or consumer otherwise.
//
// cfg is expected to already be defaulted and validated (see streamStart).
func (s *stream) run(ctx context.Context, a arm.Arm, seed []referenceframe.Input, cfg streamConfig) error {
	return newCoordinator(a, &cfg, s.trace).run(ctx, s.jointPositionsCh, seed).wait()
}

func toInputs(v interface{}) ([]referenceframe.Input, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected a list of joint positions, got %T", v)
	}
	out := make([]referenceframe.Input, len(arr))
	for i, e := range arr {
		f, ok := e.(float64)
		if !ok {
			return nil, fmt.Errorf("joint position %d is not a number (got %T)", i, e)
		}
		out[i] = referenceframe.Input(f)
	}
	return out, nil
}
