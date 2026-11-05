package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/streaming"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/contextutils/metadata"
)

const (
	streamKeyArm                   = "arm"
	streamKeyDiagnosticsWindowSecs = "diagnostics_window_secs"
	streamKeyLastWindowDetails     = "last_window_details"
)

// stream records one running TempStreamArmJointPositions session, so that the stream_status
// DoCommand can report on it while the session's owning RPC call is still blocked running it.
type stream struct {
	armName string

	// cancel aborts the session immediately, dropping any buffered trajectory that hasn't
	// reached the arm. Used by Close to tear down sessions still running at shutdown.
	cancel context.CancelFunc

	// done is closed by TempStreamArmJointPositions as the last thing it does before returning,
	// however the session ended. abortStreams uses it to wait for a canceled session to finish
	// tearing down.
	done chan struct{}

	opts streaming.StreamOptions

	diagnostics *diagnostics.SingleSessionDiagnostics
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
	if opts.MaxTargetsDroppedInARowDueToTrajexExtendError, err = streaming.MaxTargetsDroppedInARowDueToTrajexExtendError(extra); err != nil {
		return fmt.Errorf("invalid streaming options: %w", err)
	}
	if opts.ForceRebaseIfExtendLatencyExceedsMs, err = streaming.ForceRebaseIfExtendLatencyExceedsMs(extra); err != nil {
		return fmt.Errorf("invalid streaming options: %w", err)
	}
	if err := opts.Validate(); err != nil {
		return fmt.Errorf("invalid streaming options: %w", err)
	}
	metadataTags, err := ms.streamDiagnosticsMetadataTags(ctx)
	if err != nil {
		return fmt.Errorf("invalid request metadata: %w", err)
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

	// A cancelable ctx lets Close end the session from another goroutine (via s.cancel), even
	// though this call otherwise just runs to completion on the caller's own ctx.
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
		ms.unregisterStream(armName)
		close(s.done)
	}()

	sessionStart := time.Now()
	err = streaming.Run(streamCtx, ms.logger, a, opts, targets, seed, diag, responses)
	if err != nil {
		ms.logger.CWarnf(streamCtx, "arm streaming session for %q ended with error: %v", armName, err)
	}
	ms.logger.Infow("arm streaming session stats", "arm", armName, "options", opts, "stats", diag.Stats())
	if werr := ms.writeStreamDiagnostics(armName, opts, diag, sessionStart, metadataTags, err); werr != nil {
		ms.logger.Warnf("couldn't write arm streaming diagnostics: %v", werr)
	}
	return err
}

type streamDiagnosticsFile struct {
	Arm               string                                     `json:"arm"`
	StartTime         time.Time                                  `json:"start_time"`
	EndTime           time.Time                                  `json:"end_time"`
	Error             string                                     `json:"error,omitempty"`
	Options           streaming.StreamOptions                    `json:"options"`
	Stats             diagnostics.SingleSessionStats             `json:"stats"`
	LastWindowDetails diagnostics.SingleSessionLastWindowDetails `json:"last_window_details"`
}

func (ms *builtIn) writeStreamDiagnostics(
	armName string, opts streaming.StreamOptions, diag *diagnostics.SingleSessionDiagnostics, start time.Time, metadataTags []string, sessionErr error,
) error {
	if ms.conf == nil || ms.conf.StreamDiagnosticsFilePath == "" {
		return nil
	}

	end := time.Now()
	doc := streamDiagnosticsFile{
		Arm:               armName,
		StartTime:         start,
		EndTime:           end,
		Options:           opts,
		Stats:             diag.Stats(),
		LastWindowDetails: diag.LastWindowDetails(),
	}
	typeTag := "tag=arm-stream-diagnostics"
	errSuffix := ""
	if sessionErr != nil {
		doc.Error = sessionErr.Error()
		typeTag += "-err"
		errSuffix = "-err"
	}
	bytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	dirParts := append([]string{ms.conf.StreamDiagnosticsFilePath, typeTag, "tag=" + armName}, metadataTags...)
	dir := filepath.Join(dirParts...)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	fn := filepath.Join(dir, fmt.Sprintf("stream-diagnostics-%s-%s-ms-%d%s.json",
		armName, end.Format(time.RFC3339), end.Sub(start).Milliseconds(), errSuffix))

	ms.logger.Infof("writing arm streaming diagnostics to %s", fn)
	return os.WriteFile(fn, bytes, 0o600)
}

func (ms *builtIn) streamDiagnosticsMetadataTags(ctx context.Context) ([]string, error) {
	if ms.conf == nil {
		return nil, nil
	}
	var tags []string
	for _, key := range ms.conf.StreamDiagnosticsMetadataTagKeys {
		value, ok := metadata.Get(ctx, key)
		if !ok || value == "" {
			continue
		}
		if strings.ContainsAny(value, "/\\\x00") {
			return nil, fmt.Errorf("value of metadata key %q contains a path separator", key)
		}
		tags = append(tags, "tag="+value)
	}
	return tags, nil
}

func (ms *builtIn) streamStatus(armName string) map[string]any {
	ms.streamMu.RLock()
	s, ok := ms.streams[armName]
	ms.streamMu.RUnlock()
	if !ok {
		return map[string]any{}
	}
	status := map[string]any{streamKeyDiagnosticsWindowSecs: s.opts.DiagnosticsWindowSecs}
	if s.opts.DiagnosticsWindowSecs > 0 {
		status[streamKeyLastWindowDetails] = s.diagnostics.LastWindowDetails()
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
