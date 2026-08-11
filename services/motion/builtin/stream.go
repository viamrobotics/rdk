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
	streamKeyOk      = "ok"
)

// stream manages a single arm-streaming session across start/push/abort/status
// DoCommands.
type stream struct {
	logger  logging.Logger
	armName string

	// jpCh is the channel on which joint positions received via the streamPush
	// DoCommand are passed to the background goroutine running the session.
	// Closing it signals the session to finish sending the queued targets to the
	// arm, then teardown cleanly.
	jpCh chan streaming.JointPositionsChItem

	// Currently, the stream object is used by unary Do commands simulating a stream
	// interface. The unary interface can be abused in more ways, such as conccurent
	// streamPush calls, or streamPush after a streamFlush.
	// We use a mutex to serialize streamPush and streamFlush to simulate a streaming
	// interface.
	opMu sync.Mutex
	// closed reports that flush has closed jpCh. Guarded by opMu.
	closed bool

	// cancel signals the background goroutine running the session to abort
	// immediately, without sending the queued targets to the arm. It is one of
	// several ways the session can end (flush and errors are the others).
	cancel context.CancelFunc

	// done is closed by the background goroutine as the last thing it does on
	// exit, whichever way the session ended (flush, abort, or error).
	done chan struct{}

	// err is the error (if any) that caused the session to end. It is only
	// safe to read after done is closed.
	err error
}

func (s *stream) finished() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// send delivers one push's targets onto the session channel, in order. It holds opMu for the
// whole batch, so a push is atomic with respect to other pushes and to flush.
func (s *stream) send(ctx context.Context, targets []streaming.JointPositionsChItem) error {
	if !s.opMu.TryLock() {
		return errors.New("overlapping stream operations: pushes and flush must be issued sequentially")
	}
	defer s.opMu.Unlock()
	if s.closed {
		return errors.New("streaming session is flushing or has ended; no further targets are accepted")
	}

	for _, t := range targets {
		// Each send returns once the executor accepts the target, the session ends, or ctx
		// is done.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			if s.err != nil {
				return fmt.Errorf("streaming session ended: %w", s.err)
			}
			return errors.New("streaming session ended")
		case s.jpCh <- t:
		}
	}
	return nil
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
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	go func() {
		err := streaming.Run(streamCtx, a, opts, s.jpCh, seed)
		s.err = err
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

	return s.send(ctx, jpChItem)
}

func (ms *builtIn) streamFlush(ctx context.Context) (map[string]any, error) {
	// Wait for the session to finish outside the lock, since the wait can block.
	ms.streamMu.RLock()
	s := ms.stream
	ms.streamMu.RUnlock()
	if s == nil {
		return nil, fmt.Errorf("no streaming session is running; call %s first", DoStreamStart)
	}

	// Close jpCh to signal the background goroutine to stop accepting targets and drain the
	// remaining trajectory to the arm. Taking opMu first waits out an in-flight push and bars
	// new ones, so no send can race the close; the closed flag makes the close idempotent and
	// turns late pushes into a clear error.
	s.opMu.Lock()
	if !s.closed {
		s.closed = true
		close(s.jpCh)
	}
	s.opMu.Unlock()

	// Wait for the session to finish.
	select {
	case <-s.done:
	case <-ctx.Done():
		// The caller gave up waiting; the session keeps draining on its own.
		return map[string]any{streamKeyRunning: true}, nil
	}

	status := map[string]any{streamKeyRunning: false}
	if s.err != nil {
		status[streamKeyError] = s.err.Error()
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
	if s.err != nil {
		status[streamKeyError] = s.err.Error()
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
	if finished && ms.stream.err != nil {
		status[streamKeyError] = ms.stream.err.Error()
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
		return map[string]interface{}{streamKeyOk: 1}, true, nil
	}

	if req, ok := cmd[DoStreamPush]; ok {
		targets, err := parseStreamTargets(req)
		if err != nil {
			return nil, true, err
		}
		if err := ms.streamPush(ctx, targets); err != nil {
			return nil, true, err
		}
		return map[string]interface{}{streamKeyOk: 1}, true, nil
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
	// Start from the defaults; any options in the request override them.
	opts := streaming.NewDefaultOptions()

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
