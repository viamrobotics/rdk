package builtin

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/utils"
)

// Keys used in arm-streaming DoCommand requests and responses. streamKeyArm
// names the arm both in DoStreamStart requests and in DoStreamStatus responses.
const (
	streamKeyArm     = "arm"
	streamKeyOptions = "options"
	streamKeyRunning = "running"
	streamKeyError   = "error"
)

// stream manages a single arm-streaming session across start/push/abort/status
// DoCommands.
type stream struct {
	logger  logging.Logger
	armName string

	// jpCh is the channel on which joint positions received via the streamPush
	// DoCommand are passed to the background goroutine running the session.
	jpCh chan streaming.JointPositionsChItem

	// flushCh is closed (at most once, via flushOnce) to signal the background
	// goroutine running the session to finish sending the queued targets to the
	// arm, then teardown cleanly.
	flushCh   chan struct{}
	flushOnce sync.Once

	// cancel is called to signal the background goroutine running the session
	// to abort immediately, without sending the queued targets to the arm.
	cancel context.CancelFunc

	// done is closed once the session has ended.
	done chan struct{}

	// resultErr is the error (if any) that caused the session to end.
	// It should only be read after done is observed closed.
	resultErr error
}

func (s *stream) finished() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// send delivers one target onto the session channel.
func (s *stream) send(ctx context.Context, t streaming.JointPositionsChItem) error {
	// jpCh is never closed (sessions end via flushCh or cancellation), so this
	// returns once the executor accepts the target, the session ends, or ctx is done.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		if s.resultErr != nil {
			return fmt.Errorf("streaming session ended: %w", s.resultErr)
		}
		return errors.New("streaming session ended")
	case s.jpCh <- t:
		return nil
	}
}

func (ms *builtIn) streamStart(
	ctx context.Context,
	armName string,
	opts streaming.StreamOptions,
) error {
	ms.streamMu.Lock()
	defer ms.streamMu.Unlock()

	if s := ms.stream; s != nil {
		if !s.finished() {
			return fmt.Errorf("a stream is already running or still shutting down; call %s or %s first",
				DoStreamFlush, DoStreamAbort)
		}
		ms.stream = nil
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

	seed, err := a.JointPositions(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to read seed joint positions from %q: %w", armName, err)
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	s := &stream{
		logger:  ms.logger.Sublogger("arm_streaming"),
		armName: armName,
		jpCh:    make(chan streaming.JointPositionsChItem),
		flushCh: make(chan struct{}),
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	go func() {
		err := streaming.Run(streamCtx, a, &opts, s.jpCh, s.flushCh, seed)
		s.resultErr = err
		if err != nil {
			s.logger.CWarnf(streamCtx, "arm streaming session ended with error: %v", err)
		}
		close(s.done)
	}()

	ms.stream = s
	return nil
}

func (ms *builtIn) streamPush(ctx context.Context, jpChItem []streaming.JointPositionsChItem) error {
	// Push to the channel outside the lock, since the channel send can block.
	ms.streamMu.RLock()
	s := ms.stream
	ms.streamMu.RUnlock()
	if s == nil {
		return fmt.Errorf("no streaming session is running; call %s first", DoStreamStart)
	}

	for _, jpItem := range jpChItem {
		if err := s.send(ctx, jpItem); err != nil {
			return err
		}
	}
	return nil
}

func (ms *builtIn) streamFlush(ctx context.Context) (map[string]any, error) {
	// Wait for the session to finish outside the lock, since the wait can block.
	ms.streamMu.RLock()
	s := ms.stream
	ms.streamMu.RUnlock()
	if s == nil {
		return nil, fmt.Errorf("no streaming session is running; call %s first", DoStreamStart)
	}

	// Signal the background goroutine to stop accepting targets and drain the
	// remaining trajectory to the arm.
	s.flushOnce.Do(func() { close(s.flushCh) })

	// Wait for the session to finish.
	select {
	case <-s.done:
	case <-ctx.Done():
		// The caller gave up waiting; the session keeps draining on its own.
		return map[string]any{streamKeyRunning: true}, nil
	}

	status := map[string]any{streamKeyRunning: false}
	if s.resultErr != nil {
		status[streamKeyError] = s.resultErr.Error()
	}
	return status, nil
}

func (ms *builtIn) streamAbort(ctx context.Context) map[string]any {
	// Wait for the session to finish outside the lock, since the wait can block.
	ms.streamMu.Lock()
	s := ms.stream
	ms.streamMu.Unlock()
	if s == nil {
		return map[string]any{streamKeyRunning: false}
	}

	// Signal the background goroutine to abort the session.
	s.cancel()

	// Wait for the session to finish.
	select {
	case <-s.done:
	case <-ctx.Done():
		// The caller gave up waiting.
		return map[string]any{streamKeyRunning: true}
	}

	status := map[string]any{streamKeyRunning: false}
	if s.resultErr != nil {
		status[streamKeyError] = s.resultErr.Error()
	}
	return status
}

func (ms *builtIn) streamStatus() map[string]any {
	ms.streamMu.RLock()
	defer ms.streamMu.RUnlock()
	if ms.stream == nil {
		return map[string]any{streamKeyRunning: false}
	}

	finished := ms.stream.finished()
	status := map[string]any{
		streamKeyRunning: !finished,
		streamKeyArm:     ms.stream.armName,
	}
	if finished && ms.stream.resultErr != nil {
		status[streamKeyError] = ms.stream.resultErr.Error()
	}
	return status
}

func (ms *builtIn) handleStreamCommand(
	ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, bool, error) {
	if req, ok := cmd[DoStreamStart]; ok {
		armName, opts, err := parseStreamStart(req)
		if err != nil {
			return nil, true, err
		}
		if err := ms.streamStart(ctx, armName, opts); err != nil {
			return nil, true, err
		}
		return map[string]interface{}{}, true, nil
	}

	if req, ok := cmd[DoStreamPush]; ok {
		targets, err := parseStreamTargets(req)
		if err != nil {
			return nil, true, err
		}
		if err := ms.streamPush(ctx, targets); err != nil {
			return nil, true, err
		}
		return map[string]interface{}{}, true, nil
	}

	if _, ok := cmd[DoStreamFlush]; ok {
		status, err := ms.streamFlush(ctx)
		if err != nil {
			return nil, true, err
		}
		return status, true, nil
	}

	if _, ok := cmd[DoStreamAbort]; ok {
		return ms.streamAbort(ctx), true, nil
	}

	if _, ok := cmd[DoStreamStatus]; ok {
		return ms.streamStatus(), true, nil
	}

	return nil, false, nil
}

func parseStreamStart(req interface{}) (string, streaming.StreamOptions, error) {
	var opts streaming.StreamOptions
	m, err := utils.AssertType[map[string]interface{}](req)
	if err != nil {
		return "", opts, fmt.Errorf("%s expects an object", DoStreamStart)
	}

	armName, _ := m[streamKeyArm].(string)
	if armName == "" {
		return "", opts, fmt.Errorf("%s requires a %q field", DoStreamStart, streamKeyArm)
	}

	if rawOpts, ok := m[streamKeyOptions]; ok {
		if err := streaming.ParseStreamOptions(rawOpts, &opts); err != nil {
			return "", opts, fmt.Errorf("invalid streaming options: %w", err)
		}
	}

	// Validate here so bad options fail the DoStreamStart synchronously, rather than
	// spawning a session that is already dead.
	opts.ApplyDefaults()
	if err := opts.Validate(); err != nil {
		return "", opts, fmt.Errorf("invalid streaming options: %w", err)
	}

	return armName, opts, nil
}

func parseStreamTargets(req interface{}) ([]streaming.JointPositionsChItem, error) {
	arr, ok := req.([]interface{})
	if !ok || len(arr) == 0 {
		return nil, fmt.Errorf("%s expects a non-empty list of joint-position vectors", DoStreamPush)
	}

	targets := make([]streaming.JointPositionsChItem, len(arr))
	for i, e := range arr {
		vec, err := toInputs(e)
		if err != nil {
			return nil, fmt.Errorf("target %d: %w", i, err)
		}
		targets[i] = streaming.JointPositionsChItem{Positions: vec}
	}
	return targets, nil
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
