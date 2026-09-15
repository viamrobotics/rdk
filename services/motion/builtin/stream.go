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

// Keys used in the stream_status DoCommand request and response. streamKeyArm names the arm
// both in the request and in the response.
const (
	streamKeyArm               = "arm"
	streamKeyOptions           = "options"
	streamKeyRunning           = "running"
	streamKeyError             = "error"
	streamKeyLastWindowDetails = "last_window_details"
)

// activeStream records one running (or just-finished) StreamArmJointPositions session, so that
// the stream_status DoCommand can report on it concurrently while the session's owning RPC call
// is still blocked running it.
type activeStream struct {
	armName string
	opts    streaming.StreamOptions

	// cancel aborts the session immediately, dropping any buffered trajectory that hasn't
	// reached the arm. Used by Close to tear down sessions still running at shutdown.
	cancel context.CancelFunc

	// done is closed by StreamArmJointPositions as the last thing it does before returning,
	// however the session ended (drained normally, aborted, or errored).
	done chan struct{}

	// err is the error (if any) that ended the session. Only safe to read after done is closed.
	err error

	diagnostics *diagnostics.SingleSessionDiagnostics
}

func (s *activeStream) finished() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// registerStream reserves armName for a new session, replacing a previous entry only if it has
// already finished. It returns an error if a session for armName is still running.
func (ms *builtIn) registerStream(armName string, s *activeStream) error {
	ms.streamMu.Lock()
	defer ms.streamMu.Unlock()
	if existing, ok := ms.streams[armName]; ok && !existing.finished() {
		return fmt.Errorf("a stream is already running for arm %q", armName)
	}
	if ms.streams == nil {
		ms.streams = make(map[string]*activeStream)
	}
	ms.streams[armName] = s
	return nil
}

// abortStreams cancels every session still running, and waits for each to finish tearing down.
func (ms *builtIn) abortStreams(ctx context.Context) {
	ms.streamMu.RLock()
	streams := make([]*activeStream, 0, len(ms.streams))
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

// StreamArmJointPositions implements motion.ArmJointPositionStreamer. It derives and paces a
// trajectory to armName from targets, blocking until targets is closed and the derived
// trajectory has finished executing on the arm, or until ctx is canceled.
func (ms *builtIn) StreamArmJointPositions(
	ctx context.Context,
	armName string,
	opts motion.StreamOptions,
	targets <-chan []referenceframe.Input,
	extra map[string]interface{},
) (err error) {
	streamOpts := streaming.NewDefaultOptions()
	if opts.TargetRunwayInArmMs != nil {
		streamOpts.TargetRunwayInArmMs = int(*opts.TargetRunwayInArmMs)
	}
	if opts.SendToArmIntervalMs != nil {
		streamOpts.SendToArmIntervalMs = int(*opts.SendToArmIntervalMs)
	}
	if opts.VelLimitDegPerSec != nil {
		streamOpts.VelLimitDegPerSec = *opts.VelLimitDegPerSec
	}
	if opts.AccelLimitDegPerSec2 != nil {
		streamOpts.AccelLimitDegPerSec2 = *opts.AccelLimitDegPerSec2
	}
	if opts.DiagnosticsWindowMs != nil {
		streamOpts.DiagnosticsWindowMs = int(*opts.DiagnosticsWindowMs)
	}
	if err := streamOpts.Validate(); err != nil {
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

	var diag *diagnostics.SingleSessionDiagnostics
	if streamOpts.DiagnosticsWindowMs > 0 {
		diag = diagnostics.New(time.Duration(streamOpts.DiagnosticsWindowMs) * time.Millisecond)
	}

	// Derive a cancelable ctx so Close (or a concurrent abort) can end the session even though
	// this call otherwise just runs to completion on the caller's ctx.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	s := &activeStream{
		armName:     armName,
		opts:        streamOpts,
		cancel:      cancel,
		done:        make(chan struct{}),
		diagnostics: diag,
	}
	if err := ms.registerStream(armName, s); err != nil {
		return err
	}
	defer func() {
		s.err = err
		close(s.done)
	}()

	seed, err := a.JointPositions(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to read seed joint positions from %q: %w", armName, err)
	}

	// streaming.Run wants one JointPositionsChItem at a time; fan targets out onto that shape.
	jpCh := make(chan streaming.JointPositionsChItem)
	go func() {
		defer close(jpCh)
		for {
			select {
			case <-runCtx.Done():
				return
			case t, ok := <-targets:
				if !ok {
					return
				}
				select {
				case jpCh <- streaming.JointPositionsChItem{Positions: t}:
				case <-runCtx.Done():
					return
				}
			}
		}
	}()

	err = streaming.Run(runCtx, a, streamOpts, jpCh, seed, diag)
	if err != nil {
		ms.logger.CWarnf(runCtx, "arm streaming session for %q ended with error: %v", armName, err)
	}
	if diag != nil {
		ms.logger.Infow("arm streaming session stats", "arm", armName, "options", streamOpts, "stats", diag.Stats())
	}
	return err
}

func (ms *builtIn) streamStatus(armName string, includeLastWindowDetails bool) map[string]any {
	ms.streamMu.RLock()
	s, ok := ms.streams[armName]
	ms.streamMu.RUnlock()
	if !ok {
		return map[string]any{streamKeyRunning: false}
	}

	finished := s.finished()
	status := map[string]any{
		streamKeyRunning: !finished,
		streamKeyArm:     s.armName,
		streamKeyOptions: s.opts,
	}
	if includeLastWindowDetails && s.diagnostics != nil {
		status[streamKeyLastWindowDetails] = s.diagnostics.LastWindowDetails()
	}
	if finished && s.err != nil {
		status[streamKeyError] = s.err.Error()
	}
	return status
}

func (ms *builtIn) handleStreamCommand(
	ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, bool, error) {
	if req, ok := cmd[DoStreamStatus]; ok {
		armName, includeLastWindowDetails, err := parseStreamStatus(req)
		if err != nil {
			return nil, true, err
		}
		return ms.streamStatus(armName, includeLastWindowDetails), true, nil
	}

	return nil, false, nil
}

// parseStreamStatus decodes the stream_status request, requiring the target arm's name.
func parseStreamStatus(req interface{}) (string, bool, error) {
	m, err := utils.AssertType[map[string]interface{}](req)
	if err != nil {
		return "", false, fmt.Errorf("%s expects an object with an %q field", DoStreamStatus, streamKeyArm)
	}
	armName, _ := m[streamKeyArm].(string)
	if armName == "" {
		return "", false, fmt.Errorf("%s requires an %q field", DoStreamStatus, streamKeyArm)
	}
	includeLastWindowDetails, _ := m[streamKeyLastWindowDetails].(bool)
	return armName, includeLastWindowDetails, nil
}
