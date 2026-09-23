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
	streamKeyLastWindowDetails = "last_window_details"
)

// stream records one running StreamArmJointPositions session, so that the stream_status
// DoCommand can report on it concurrently while the session's owning RPC call is still blocked
// running it. A session is removed from streams as soon as it finishes, so a *stream found there
// is always still running.
type stream struct {
	armName string

	// cancel aborts the session immediately, dropping any buffered trajectory that hasn't
	// reached the arm. Used by Close to tear down sessions still running at shutdown.
	cancel context.CancelFunc

	// done is closed by StreamArmJointPositions as the last thing it does before returning,
	// however the session ended (drained normally, aborted, or errored). abortStreams uses it to
	// wait for a canceled session to finish tearing down.
	done chan struct{}

	opts streaming.StreamOptions

	diagnostics *diagnostics.SingleSessionDiagnostics
}

// registerStream reserves armName for a new session. It returns an error if a session for
// armName is already running.
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

// unregisterStream removes s from streams once it has finished executing, so that a session
// that has ended does not linger in the map indefinitely. It only removes s itself, in case a
// new session for armName has already been registered in its place.
func (ms *builtIn) unregisterStream(armName string, s *stream) {
	ms.streamMu.Lock()
	defer ms.streamMu.Unlock()
	if ms.streams[armName] == s {
		delete(ms.streams, armName)
	}
}

// abortStreams cancels every session still running, and waits for each to finish tearing down.
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

// StreamArmJointPositions implements motion.Service. It derives and paces a trajectory to
// armName from targets, blocking until targets is closed and the derived trajectory has
// finished executing on the arm, or until ctx is canceled.
func (ms *builtIn) StreamArmJointPositions(
	ctx context.Context,
	armName string,
	streamOpts motion.StreamOptions,
	targets <-chan []referenceframe.Input,
	extra map[string]interface{},
) (err error) {
	var opts streaming.StreamOptions
	opts.From(streamOpts)
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

	diag := diagnostics.New(time.Duration(opts.DiagnosticsWindowSecs) * time.Second)

	// Derive a cancelable ctx so cancel (stashed on s below) lets Close end the session from
	// another goroutine, even though this call otherwise just runs to completion on the
	// caller's own ctx.
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	s := &stream{
		armName:     armName,
		cancel:      cancel,
		done:        make(chan struct{}),
		opts:        opts,
		diagnostics: diag,
	}
	if err := ms.registerStream(armName, s); err != nil {
		return err
	}
	defer func() {
		ms.unregisterStream(armName, s)
		close(s.done)
	}()

	err = streaming.Run(streamCtx, a, opts, targets, seed, diag)
	if err != nil {
		ms.logger.CWarnf(streamCtx, "arm streaming session for %q ended with error: %v", armName, err)
	}
	ms.logger.Infow("arm streaming session stats", "arm", armName, "options", opts, "stats", diag.Stats())
	return err
}

func (ms *builtIn) streamStatus(armName string, includeLastWindowDetails bool) (map[string]any, error) {
	ms.streamMu.RLock()
	s, ok := ms.streams[armName]
	ms.streamMu.RUnlock()
	if !ok {
		return map[string]any{streamKeyRunning: false}, nil
	}

	if includeLastWindowDetails && s.opts.DiagnosticsWindowSecs <= 0 {
		return nil, fmt.Errorf(
			"%s was requested but diagnostics_window_secs is not positive, so it is not being retained",
			streamKeyLastWindowDetails,
		)
	}

	status := map[string]any{
		streamKeyRunning: true,
		streamKeyArm:     s.armName,
		streamKeyOptions: s.opts,
	}
	if includeLastWindowDetails {
		status[streamKeyLastWindowDetails] = s.diagnostics.LastWindowDetails()
	}
	return status, nil
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
		status, err := ms.streamStatus(armName, includeLastWindowDetails)
		return status, true, err
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
