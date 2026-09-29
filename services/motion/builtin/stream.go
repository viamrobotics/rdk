package builtin

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/utils"
)

const (
	streamKeyArm                   = "arm"
	streamKeyRunning               = "running"
	streamKeyDiagnosticsWindowSecs = "diagnostics_window_secs"
	streamKeyLastWindowDetails     = "last_window_details"
)

// stream records one running TempStreamArmJointPositions session, so that stream_status can
// tell a session is currently running (options and diagnostics are no longer read off it:
// they live in ms.streamDiagnostics, which outlives the session — see armDiagnostics).
type stream struct {
	armName string

	// cancel aborts the session immediately, dropping any buffered trajectory that hasn't
	// reached the arm. Used by Close to tear down sessions still running at shutdown.
	cancel context.CancelFunc

	// done is closed by TempStreamArmJointPositions as the last thing it does before returning,
	// however the session ended. abortStreams uses it to wait for a canceled session to finish
	// tearing down.
	done chan struct{}
}

func (ms *builtIn) registerStream(armName string, s *stream) error {
	ms.streamMu.Lock()
	defer ms.streamMu.Unlock()
	if _, ok := ms.streams[armName]; ok {
		return fmt.Errorf("a stream is already running for arm %q", armName)
	}
	if ms.streams == nil {
		ms.streams = make(map[string]*stream)
	}
	ms.streams[armName] = s
	return nil
}

func (ms *builtIn) unregisterStream(armName string) {
	ms.streamMu.Lock()
	defer ms.streamMu.Unlock()
	delete(ms.streams, armName)
}

// armDiagnostics returns armName's persistent diagnostics, creating them on first use.
// Diagnostics survive the session that created them (see the streamDiagnostics field and
// SingleSessionDiagnostics): a later session on the same arm continues appending to the same
// retained window rather than starting fresh, with the window updated to whatever this session
// configures. Stats() is reset so it keeps describing only the session that's starting.
func (ms *builtIn) armDiagnostics(armName string, window time.Duration) *diagnostics.SingleSessionDiagnostics {
	ms.streamMu.Lock()
	defer ms.streamMu.Unlock()
	if ms.streamDiagnostics == nil {
		ms.streamDiagnostics = make(map[string]*diagnostics.SingleSessionDiagnostics)
	}
	diag, ok := ms.streamDiagnostics[armName]
	if !ok {
		diag = diagnostics.New(window)
		ms.streamDiagnostics[armName] = diag
		return diag
	}
	diag.SetWindow(window)
	diag.ResetStats()
	return diag
}

func (ms *builtIn) abortStreams(ctx context.Context) {
	ms.streamMu.RLock()
	streams := make([]*stream, 0, len(ms.streams))
	for _, s := range ms.streams {
		streams = append(streams, s)
	}
	ms.streamMu.RUnlock()

	for _, s := range streams {
		s.cancel()
		select {
		case <-s.done:
		case <-ctx.Done():
			return
		}
	}
}

// TempStreamArmJointPositions implements motion.Service. It derives and paces a trajectory to
// armName from targets, blocking until targets is closed and the derived trajectory has finished
// executing on the arm, or until ctx is canceled.
func (ms *builtIn) TempStreamArmJointPositions(
	ctx context.Context,
	armName string,
	streamOpts motion.TempStreamOptions,
	targets <-chan []referenceframe.Input,
	responses chan<- motion.TempStreamResponse,
	extra map[string]interface{},
) (err error) {
	opts := streaming.NewStreamOptions(streamOpts)
	if err := opts.Validate(); err != nil {
		return fmt.Errorf("invalid streaming options: %w", err)
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

	diag := ms.armDiagnostics(armName, time.Duration(opts.DiagnosticsWindowSecs)*time.Second)

	// A cancelable ctx lets Close end the session from another goroutine (via s.cancel), even
	// though this call otherwise just runs to completion on the caller's own ctx.
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	s := &stream{
		armName: armName,
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	if err := ms.registerStream(armName, s); err != nil {
		return err
	}
	defer func() {
		ms.unregisterStream(armName)
		close(s.done)
	}()

	err = streaming.Run(streamCtx, a, opts, targets, seed, diag, responses)
	if err != nil {
		ms.logger.CWarnf(streamCtx, "arm streaming session for %q ended with error: %v", armName, err)
	}
	ms.logger.Infow("arm streaming session stats", "arm", armName, "options", opts, "stats", diag.Stats())
	return err
}

// streamStatus reports armName's current running state plus whatever diagnostics are
// retained for it, which may be from a session that already ended: diagnostics persist past
// the session that recorded them (see the streamDiagnostics field), so a crashed or finished
// pass's last_window_details remains visible until a later session on the same arm ages it
// out. An empty result means armName has never streamed at all, as distinct from having
// streamed and stopped.
func (ms *builtIn) streamStatus(armName string) map[string]any {
	ms.streamMu.RLock()
	_, running := ms.streams[armName]
	diag, hasDiag := ms.streamDiagnostics[armName]
	ms.streamMu.RUnlock()
	if !running && !hasDiag {
		return map[string]any{}
	}
	status := map[string]any{streamKeyRunning: running}
	windowSecs := 0
	if hasDiag {
		windowSecs = diag.WindowSecs()
	}
	status[streamKeyDiagnosticsWindowSecs] = windowSecs
	if windowSecs > 0 {
		status[streamKeyLastWindowDetails] = diag.LastWindowDetails()
	}
	return status
}

func (ms *builtIn) handleStreamCommand(
	ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, bool, error) {
	if req, ok := cmd[DoStreamStatus]; ok {
		armName, err := parseStreamStatus(req)
		if err != nil {
			return nil, true, err
		}
		return ms.streamStatus(armName), true, nil
	}

	return nil, false, nil
}

func parseStreamStatus(req interface{}) (string, error) {
	m, err := utils.AssertType[map[string]interface{}](req)
	if err != nil {
		return "", fmt.Errorf("%s expects an object with an %q field", DoStreamStatus, streamKeyArm)
	}

	armName, _ := m[streamKeyArm].(string)
	if armName == "" {
		return "", fmt.Errorf("%s requires an %q field", DoStreamStatus, streamKeyArm)
	}
	return armName, nil
}
