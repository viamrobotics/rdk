package builtin

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/go-viper/mapstructure/v2"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

const (
	defaultBufferAheadInArmMs   = 100
	defaultSendToArmIntervalMs  = 10
	defaultVelLimitDegPerSec    = 10.0
	defaultAccelLimitDegPerSec2 = 10.0
)

// streamTarget is one live joint-space waypoint fed to the streaming executor.
type streamTarget struct {
	// Positions are the target joint positions for this waypoint.
	Positions []referenceframe.Input

	// Flush ends the current trajex session and arm stream at this point and
	// starts a fresh one. Use it at segment boundaries where the arm may briefly
	// stop (e.g. between sanding strokes).
	Flush bool
}

// streamConfig tunes the streaming executor.
type streamConfig struct {
	// BufferAheadInArmMs is the lookahead window (ms) of points sampled out of
	// trajex that we aim to keep buffered inside the arm resource.
	BufferAheadInArmMs int `json:"buffer_ahead_in_arm_ms"`

	// SendToArmIntervalMs is the interval (ms) at which points are sampled out of
	// trajex and sent to the arm to top the arm's buffer back up to
	// BufferAheadInArmMs.
	// TODO: Replace this with querying the arm's properties API.
	SendToArmIntervalMs int `json:"send_to_arm_interval_ms"`

	// VelLimitDegPerSec / AccelLimitDegPerSec2 are the per-joint limits the trajex
	// session is built with.
	// TODO: Replace these with querying the arm's properties API.
	VelLimitDegPerSec    float64 `json:"vel_limit_deg_per_sec"`
	AccelLimitDegPerSec2 float64 `json:"accel_limit_deg_per_sec2"`
}

// Validate returns an error if any streamConfig field is invalid.
func (c *streamConfig) Validate() error {
	if c.BufferAheadInArmMs < 0 {
		return errors.New("streaming: buffer_ahead_in_arm_ms must be non-negative")
	}
	if c.SendToArmIntervalMs < 0 {
		return errors.New("streaming: send_to_arm_interval_ms must be non-negative")
	}
	if c.SendToArmIntervalMs == 0 {
		return errors.New("streaming: send_to_arm_interval_ms must be positive")
	}
	if c.VelLimitDegPerSec < 0 {
		return errors.New("streaming: vel_limit_deg_per_sec must be non-negative")
	}
	if c.AccelLimitDegPerSec2 < 0 {
		return errors.New("streaming: accel_limit_deg_per_sec2 must be non-negative")
	}
	return nil
}

// ApplyDefaults fills any zero-valued streamConfig field with its default.
func (c *streamConfig) ApplyDefaults() {
	if c.BufferAheadInArmMs == 0 {
		c.BufferAheadInArmMs = defaultBufferAheadInArmMs
	}
	if c.SendToArmIntervalMs == 0 {
		c.SendToArmIntervalMs = defaultSendToArmIntervalMs
	}
	if c.VelLimitDegPerSec == 0 {
		c.VelLimitDegPerSec = defaultVelLimitDegPerSec
	}
	if c.AccelLimitDegPerSec2 == 0 {
		c.AccelLimitDegPerSec2 = defaultAccelLimitDegPerSec2
	}
}

// stream manages a single arm-streaming session: a goroutine runs the
// executor (run) while DoCommand calls push joint-space targets onto targetsCh
// across many requests.
//
// The session outlives individual DoCommand requests, so its goroutine runs on
// a background context (cancelled via cancel) rather than a request context.
type stream struct {
	logger  logging.Logger
	armName string
	dof     int

	targetsCh chan streamTarget

	cancel context.CancelFunc
	done   chan struct{} // closed when StreamJointTargets returns

	// targetsClosed guards against double-closing targetsCh. Only written while
	// holding builtIn.streamMu for writing.
	targetsClosed bool

	// resultErr is set exactly once, before done is closed, so it is safe to read
	// in any branch guarded by a receive on done.
	resultErr error

	pushCount atomic.Int64
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
			return fmt.Errorf("a streaming session is already running; call %s first", DoStreamStop)
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

	sessCtx, cancel := context.WithCancel(context.Background())
	sp := &stream{
		logger:    ms.logger.Sublogger("arm_streaming"),
		armName:   armName,
		dof:       len(seed),
		targetsCh: make(chan streamTarget),
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	go func() {
		err := sp.run(sessCtx, a, seed, cfg)
		sp.resultErr = err
		if err != nil {
			sp.logger.CWarnf(sessCtx, "arm streaming session ended with error: %v", err)
		}
		close(sp.done)
	}()

	ms.stream = sp
	return nil
}

func (ms *builtIn) streamPush(ctx context.Context, targets []streamTarget) (int, error) {
	ms.streamMu.RLock()
	defer ms.streamMu.RUnlock()

	sp := ms.stream
	if sp == nil {
		return 0, fmt.Errorf("no streaming session is running; call %s first", DoStreamStart)
	}

	sent := 0
	for _, t := range targets {
		if len(t.Positions) != sp.dof {
			return sent, fmt.Errorf("target has %d joint positions, but the arm has %d joints", len(t.Positions), sp.dof)
		}
		if err := sp.send(ctx, t); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

// streamStop ends the active session. When abort is false the targets
// channel is closed so the executor drains the remaining trajectory to the arm;
// when true the session context is cancelled to stop immediately.
func (ms *builtIn) streamStop(ctx context.Context, abort bool) map[string]any {
	ms.streamMu.RLock()
	sp := ms.stream
	ms.streamMu.RUnlock()
	if sp == nil {
		return map[string]any{"running": false}
	}

	// Abort unblocks any in-flight push via done (StreamJointTargets returns on
	// cancel). Graceful stop relies on the write lock below to serialize with
	// pushes, which take the read lock while sending.
	if abort {
		sp.cancel()
	}

	ms.streamMu.Lock()
	if ms.stream == sp {
		if !abort && !sp.targetsClosed {
			close(sp.targetsCh)
			sp.targetsClosed = true
		}
		ms.stream = nil
	}
	ms.streamMu.Unlock()

	select {
	case <-sp.done:
	case <-ctx.Done():
		// Caller gave up waiting; make sure the session still tears down.
		sp.cancel()
		<-sp.done
	}

	status := map[string]any{"running": false}
	if sp.resultErr != nil {
		status["error"] = sp.resultErr.Error()
	}
	return status
}

func (ms *builtIn) streamStatus() map[string]any {
	ms.streamMu.RLock()
	sp := ms.stream
	ms.streamMu.RUnlock()
	if sp == nil {
		return map[string]any{"running": false}
	}

	finished := false
	select {
	case <-sp.done:
		finished = true
	default:
	}
	status := map[string]any{
		"running":    !finished,
		"arm":        sp.armName,
		"push_count": sp.pushCount.Load(),
	}
	if finished && sp.resultErr != nil {
		status["error"] = sp.resultErr.Error()
	}
	return status
}

// abortStreamSession stops any running session immediately. Used on
// Close/Reconfigure to avoid leaking the executor goroutine.
func (ms *builtIn) abortStreamSession() {
	ms.streamMu.Lock()
	sp := ms.stream
	ms.stream = nil
	ms.streamMu.Unlock()
	if sp == nil {
		return
	}
	sp.cancel()
	<-sp.done
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

	if _, ok := cmd[DoStreamStop]; ok {
		abort, _ := cmd["abort"].(bool)
		return map[string]interface{}{DoStreamStop: ms.streamStop(ctx, abort)}, true, nil
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
		if err := decodeStreamConfig(rawCfg, &cfg); err != nil {
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
func parseStreamTargets(req interface{}) ([]streamTarget, error) {
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

	targets := make([]streamTarget, len(vectors))
	for i, vec := range vectors {
		targets[i] = streamTarget{Positions: vec}
	}
	return targets, nil
}

func decodeStreamConfig(raw interface{}, cfg *streamConfig) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "json",
		WeaklyTypedInput: true,
		Result:           cfg,
	})
	if err != nil {
		return err
	}
	return dec.Decode(raw)
}

// send delivers one target onto the session channel. Callers must hold
// builtIn.streamMu for reading so that a graceful stop (which closes targetsCh
// while holding the write lock) can never race with an in-flight send.
func (sp *stream) send(ctx context.Context, t streamTarget) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-sp.done:
		return fmt.Errorf("streaming session ended: %w", sp.resultErr)
	case sp.targetsCh <- t:
		sp.pushCount.Add(1)
		return nil
	}
}

// run executes the streaming session: it smooths the joint-space targets pushed
// onto targetsCh through a trajex session and flow-controls a bidirectional
// stream to the arm, seeding the first trajex session at seed.
//
// Return contract:
//   - nil once targetsCh is closed and every sampled PVAT has been drained to
//     the arm and acknowledged;
//   - the first non-context error from either the producer or consumer otherwise.
//
// cfg is expected to already be defaulted and validated (see streamStart).
func (sp *stream) run(ctx context.Context, a arm.Arm, seed []referenceframe.Input, cfg streamConfig) error {
	return newCoordinator(a, &cfg).run(ctx, sp.targetsCh, seed).wait()
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
