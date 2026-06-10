package armplanning

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils/trace"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/mlmodel"
)

// TrajGenConfig holds configuration for the trajectory generator ML model service.
type TrajGenConfig struct {
	Service                            string   `json:"service"`
	PathToleranceDeltaRads             *float64 `json:"path_tolerance_delta_rads,omitempty"`
	PathColinearizationRatio           *float64 `json:"path_colinearization_ratio,omitempty"`
	WaypointDeduplicationToleranceRads *float64 `json:"waypoint_deduplication_tolerance_rads,omitempty"`
	// VelocityLimitsRadsPerSec and AccelerationLimitsRadsPerSec2 hold one limit per joint, in the
	// trajectory's DOF order. Their length must match the plan trajectory's total DOF.
	VelocityLimitsRadsPerSec      []float64 `json:"velocity_limits_rads_per_sec,omitempty"`
	AccelerationLimitsRadsPerSec2 []float64 `json:"acceleration_limits_rads_per_sec2,omitempty"`
	SamplingFreqHz                *float64  `json:"trajectory_sampling_freq_hz,omitempty"`
}

// Validate returns the mlmodel service name as a required dependency and checks that velocity and
// acceleration limits are positive.
func (cfg *TrajGenConfig) Validate(path string) ([]string, error) {
	if len(cfg.VelocityLimitsRadsPerSec) == 0 {
		return nil, fmt.Errorf("need velocity_limits_rads_per_sec (one entry per joint) if using trajectory_generator")
	}
	for _, v := range cfg.VelocityLimitsRadsPerSec {
		if v <= 0 {
			return nil, fmt.Errorf("velocity_limits_rads_per_sec entries must be positive, got %v", v)
		}
	}
	if len(cfg.AccelerationLimitsRadsPerSec2) != len(cfg.VelocityLimitsRadsPerSec) {
		return nil, fmt.Errorf(
			"acceleration_limits_rads_per_sec2 must have one entry per joint to match velocity_limits_rads_per_sec (%d), got %d",
			len(cfg.VelocityLimitsRadsPerSec), len(cfg.AccelerationLimitsRadsPerSec2))
	}
	for _, a := range cfg.AccelerationLimitsRadsPerSec2 {
		if a <= 0 {
			return nil, fmt.Errorf("acceleration_limits_rads_per_sec2 entries must be positive, got %v", a)
		}
	}
	// The trajectory generator is now an in-process cgo backend (trajex), so there is no
	// remote mlmodel dependency to declare. The Service field is retained for config
	// compatibility but is currently unused.
	return nil, nil
}

// ToTrajGen resolves the named mlmodel service from deps and returns a TrajGen ready for use.
func (cfg *TrajGenConfig) ToTrajGen() (*TrajGen, error) {
	svc, err := newTrajGenBackend()
	if err != nil {
		return nil, err
	}
	return NewTrajGen(
		svc,
		cfg.PathToleranceDeltaRads,
		cfg.PathColinearizationRatio,
		cfg.WaypointDeduplicationToleranceRads,
		cfg.VelocityLimitsRadsPerSec,
		cfg.AccelerationLimitsRadsPerSec2,
		cfg.SamplingFreqHz,
	), nil
}

const (
	defaultTrajGenPathToleranceDeltaRads             = 0.1
	defaultTrajGenWaypointDeduplicationToleranceRads = 1e-3
	defaultTrajGenSamplingFreqHz                     = 10.0
	defaultTrajGenPathColinearizationRatio           = 0.0
)

// TrajGen holds a resolved trajectory generator ML model service along with its configuration.
type TrajGen struct {
	trajGen                            mlmodel.Service
	PathToleranceDeltaRads             float64   `json:"path_tolerance_delta_rads"`
	PathColinearizationRatio           float64   `json:"path_colinearization_ratio"`
	WaypointDeduplicationToleranceRads float64   `json:"waypoint_deduplication_tolerance_rads"`
	VelocityLimitsRadsPerSec           []float64 `json:"velocity_limits_rads_per_sec"`
	AccelerationLimitsRadsPerSec2      []float64 `json:"acceleration_limits_rads_per_sec2"`
	SamplingFreqHz                     float64   `json:"trajectory_sampling_freq_hz"`
}

func applyDefault(v *float64, def float64) float64 {
	if v == nil {
		return def
	}
	return *v
}

// TrajGenOverride holds per-call overrides for TrajGen settings. Any nil field
// means "use the already-configured value". Pass it to TrajGen.WithOverrides to
// get a modified copy.
type TrajGenOverride struct {
	PathToleranceDeltaRads             *float64  `json:"path_tolerance_delta_rads,omitempty"`
	PathColinearizationRatio           *float64  `json:"path_colinearization_ratio,omitempty"`
	WaypointDeduplicationToleranceRads *float64  `json:"waypoint_deduplication_tolerance_rads,omitempty"`
	VelocityLimitsRadsPerSec           []float64 `json:"velocity_limits_rads_per_sec,omitempty"`
	AccelerationLimitsRadsPerSec2      []float64 `json:"acceleration_limits_rads_per_sec2,omitempty"`
	SamplingFreqHz                     *float64  `json:"trajectory_sampling_freq_hz,omitempty"`
}

// WithOverrides returns a shallow copy of tg with any non-nil fields from o applied.
func (tg *TrajGen) WithOverrides(o *TrajGenOverride) *TrajGen {
	copy := *tg
	if o.PathToleranceDeltaRads != nil {
		copy.PathToleranceDeltaRads = *o.PathToleranceDeltaRads
	}
	if o.PathColinearizationRatio != nil {
		copy.PathColinearizationRatio = *o.PathColinearizationRatio
	}
	if o.WaypointDeduplicationToleranceRads != nil {
		copy.WaypointDeduplicationToleranceRads = *o.WaypointDeduplicationToleranceRads
	}
	if o.VelocityLimitsRadsPerSec != nil {
		copy.VelocityLimitsRadsPerSec = o.VelocityLimitsRadsPerSec
	}
	if o.AccelerationLimitsRadsPerSec2 != nil {
		copy.AccelerationLimitsRadsPerSec2 = o.AccelerationLimitsRadsPerSec2
	}
	if o.SamplingFreqHz != nil {
		copy.SamplingFreqHz = *o.SamplingFreqHz
	}
	return &copy
}

// NewTrajGen constructs a TrajGen from an mlmodel service and configuration fields,
// applying defaults for any nil optional values.
func NewTrajGen(
	svc mlmodel.Service,
	pathToleranceDeltaRads *float64,
	pathColinearizationRatio *float64,
	waypointDeduplicationToleranceRads *float64,
	velocityLimitsRadsPerSec []float64,
	accelerationLimitsRadsPerSec2 []float64,
	samplingFreqHz *float64,
) *TrajGen {
	return &TrajGen{
		trajGen:                            svc,
		PathToleranceDeltaRads:             applyDefault(pathToleranceDeltaRads, defaultTrajGenPathToleranceDeltaRads),
		PathColinearizationRatio:           applyDefault(pathColinearizationRatio, defaultTrajGenPathColinearizationRatio),
		WaypointDeduplicationToleranceRads: applyDefault(waypointDeduplicationToleranceRads, defaultTrajGenWaypointDeduplicationToleranceRads),
		VelocityLimitsRadsPerSec:           velocityLimitsRadsPerSec,
		AccelerationLimitsRadsPerSec2:      accelerationLimitsRadsPerSec2,
		SamplingFreqHz:                     applyDefault(samplingFreqHz, defaultTrajGenSamplingFreqHz),
	}
}

// TrajGenPlan is a motionplan.Plan enriched with the kinodynamic data produced by the trajectory
// generator service. Callers that only need joint configurations can use it as a plain Plan;
// callers that need velocities, accelerations, or timestamps can type-assert to *TrajGenPlan.
type TrajGenPlan struct {
	*motionplan.SimplePlan
	// Configurations holds per-joint positions at each trajectory sample, parallel to Trajectory().
	Configurations []*referenceframe.LinearInputs
	// Velocities holds per-joint velocities at each trajectory sample, parallel to Trajectory().
	Velocities []*referenceframe.LinearInputs
	// Accelerations holds per-joint accelerations at each sample. It is nil when the service did
	// not return acceleration data.
	Accelerations []*referenceframe.LinearInputs
	// SampleTimes holds the time (in seconds) of each sample, parallel to Trajectory().
	SampleTimes []float64
}

// TrajGenLog is a flattened, JSON-serializable snapshot of a TrajGenPlan's generated
// time-optimal trajectory, intended for debug log files. Each [][]float64 is row-major
// [n_samples][n_dof], parallel to SampleTimesSec.
type TrajGenLog struct {
	ConfigurationsRads       [][]float64 `json:"configurations_rads"`
	VelocitiesRadsPerSec     [][]float64 `json:"velocities_rads_per_sec"`
	AccelerationsRadsPerSec2 [][]float64 `json:"accelerations_rads_per_sec2,omitempty"`
	SampleTimesSec           []float64   `json:"sample_times_sec"`
}

// LogData returns a flattened, serializable view of the generated trajectory for logging and
// offline inspection. Accelerations are omitted when the generator did not produce them.
func (t *TrajGenPlan) LogData() TrajGenLog {
	flatten := func(lis []*referenceframe.LinearInputs) [][]float64 {
		out := make([][]float64, len(lis))
		for i, li := range lis {
			out[i] = li.GetLinearizedInputs()
		}
		return out
	}
	log := TrajGenLog{
		ConfigurationsRads:   flatten(t.Configurations),
		VelocitiesRadsPerSec: flatten(t.Velocities),
		SampleTimesSec:       t.SampleTimes,
	}
	if len(t.Accelerations) > 0 {
		log.AccelerationsRadsPerSec2 = flatten(t.Accelerations)
	}
	return log
}

// trajGenResult is the raw output of inferTrajGen.
type trajGenResult struct {
	configurations []*referenceframe.LinearInputs
	velocities     []*referenceframe.LinearInputs
	accelerations  []*referenceframe.LinearInputs // nil when not provided by the service
	sampleTimes    []float64
}

// trajGenLimitsMismatchError is returned by inferTrajGen when the configured per-joint velocity/
// acceleration limits don't cover the plan trajectory's DOF. PlanMotionTrajGen treats it as a
// signal to skip trajectory generation and fall back to the planned path, not as a hard error.
type trajGenLimitsMismatchError struct {
	configured, needed int
}

func (e trajGenLimitsMismatchError) Error() string {
	return fmt.Sprintf("trajectory generator configured for %d joint limits but plan trajectory has %d DOF",
		e.configured, e.needed)
}

// inferTrajGen sends the waypoints to the trajectory generator service and returns the resulting
// densely-sampled kinodynamic trajectory. Returns nil when the service indicates the component is
// already at the goal (fewer than 2 distinct waypoints after deduplication).
func inferTrajGen(
	ctx context.Context,
	fs *referenceframe.FrameSystem,
	trajAsInps []*referenceframe.LinearInputs,
	tg *TrajGen,
) (*trajGenResult, error) {
	if len(trajAsInps) == 0 {
		return &trajGenResult{}, nil
	}

	// GetSchema normalizes the trajectory to the full frame system, so dof spans every frame --
	// including arms that don't move in this plan (they ride along as constant columns). The
	// config must therefore cover all of them, and trajex optimizes over stationary DOF too.
	//
	// TODO: scope trajectory generation to only the frames that actually move (see
	// detectMovingFrames in check.go): filter the waypoints/dof and the per-joint limits down to
	// the moving frames before sending to trajex. That removes the all-arms config burden and
	// avoids generating over stationary joints.
	schema, err := trajAsInps[0].GetSchema(fs)
	if err != nil {
		return nil, err
	}

	dof := len(trajAsInps[0].GetLinearizedInputs())
	nWaypoints := len(trajAsInps)

	waypoints := make([]float64, 0, nWaypoints*dof)
	for _, li := range trajAsInps {
		waypoints = append(waypoints, li.GetLinearizedInputs()...)
	}

	// The per-joint limits must cover every DOF in the (schema-normalized) trajectory. If the
	// configured limits don't match, we can't generate a trajectory for this plan.
	if len(tg.VelocityLimitsRadsPerSec) != dof || len(tg.AccelerationLimitsRadsPerSec2) != dof {
		return nil, trajGenLimitsMismatchError{configured: len(tg.VelocityLimitsRadsPerSec), needed: dof}
	}
	velLimits := tg.VelocityLimitsRadsPerSec
	accelLimits := tg.AccelerationLimitsRadsPerSec2

	outMap, err := tg.trajGen.Infer(ctx, ml.Tensors{
		"waypoints_rads": tensor.New(
			tensor.Of(tensor.Float64),
			tensor.WithShape(nWaypoints, dof),
			tensor.WithBacking(waypoints),
		),
		"velocity_limits_rads_per_sec": tensor.New(
			tensor.Of(tensor.Float64),
			tensor.WithShape(dof),
			tensor.WithBacking(velLimits),
		),
		"acceleration_limits_rads_per_sec2": tensor.New(
			tensor.Of(tensor.Float64),
			tensor.WithShape(dof),
			tensor.WithBacking(accelLimits),
		),
		"path_tolerance_delta_rads": tensor.New(
			tensor.Of(tensor.Float64),
			tensor.WithShape(1),
			tensor.WithBacking([]float64{tg.PathToleranceDeltaRads}),
		),
		"path_colinearization_ratio": tensor.New(
			tensor.Of(tensor.Float64),
			tensor.WithShape(1),
			tensor.WithBacking([]float64{tg.PathColinearizationRatio}),
		),
		"waypoint_deduplication_tolerance_rads": tensor.New(
			tensor.Of(tensor.Float64),
			tensor.WithShape(1),
			tensor.WithBacking([]float64{tg.WaypointDeduplicationToleranceRads}),
		),
		"trajectory_sampling_freq_hz": tensor.New(
			tensor.Of(tensor.Float64),
			tensor.WithShape(1),
			tensor.WithBacking([]float64{tg.SamplingFreqHz}),
		),
	})
	if err != nil {
		return nil, err
	}

	configsTensor, ok := outMap["configurations_rads"]
	if !ok {
		// Service returns an empty map when fewer than 2 distinct waypoints remain after
		// deduplication -- the arm is already at the goal.
		return nil, nil
	}

	nSamples := configsTensor.Shape()[0]

	// Helper: convert a flat [n_samples, dof] tensor into []*LinearInputs using the schema.
	linearize := func(t *tensor.Dense) ([]*referenceframe.LinearInputs, error) {
		data := t.Data().([]float64)
		out := make([]*referenceframe.LinearInputs, nSamples)
		for i := range nSamples {
			li, err := schema.FloatsToInputs(data[i*dof : (i+1)*dof])
			if err != nil {
				return nil, err
			}
			out[i] = li
		}
		return out, nil
	}

	configs, err := linearize(configsTensor)
	if err != nil {
		return nil, err
	}

	velsTensor, ok := outMap["velocities_rads_per_sec"]
	if !ok {
		return nil, errors.New("trajectory generator service did not return velocities_rads_per_sec")
	}
	vels, err := linearize(velsTensor)
	if err != nil {
		return nil, err
	}

	times := outMap["sample_times_sec"].Data().([]float64)

	result := &trajGenResult{
		configurations: configs,
		velocities:     vels,
		sampleTimes:    times,
	}

	if accelTensor, ok := outMap["accelerations_rads_per_sec2"]; ok {
		result.accelerations, err = linearize(accelTensor)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// PlanMotionTrajGen plans a motion from a provided plan request using a trajectory generator.
func PlanMotionTrajGen(
	ctx context.Context, parentLogger logging.Logger, request *PlanRequest, trajGen *TrajGen,
) (motionplan.Plan, *PlanMeta, error) {
	logger := parentLogger.Sublogger("mp")

	start := time.Now()
	meta := &PlanMeta{}
	ctx, span := trace.StartSpan(ctx, "PlanMotion")
	defer func() {
		meta.Duration = time.Since(start)
		span.End()
	}()

	trajAsInps, err := planWaypoints(ctx, logger, request, meta)
	if err != nil {
		return nil, meta, err
	}

	logger.CInfof(ctx, "sending %d waypoints to traj-gen service", len(trajAsInps))
	tgResult, err := inferTrajGen(ctx, request.FrameSystem, trajAsInps, trajGen)
	var limitsMismatch trajGenLimitsMismatchError
	if errors.As(err, &limitsMismatch) {
		// A resource in the trajectory isn't covered by the trajectory generator config. Skip
		// trajectory generation for this plan and return the planned path unmodified.
		logger.CWarnf(ctx, "%s; skipping trajectory generation and returning the planned path", err)
		simplePlan, err := motionplan.NewSimplePlanFromTrajectory(trajAsInps, request.FrameSystem)
		if err != nil {
			return nil, meta, err
		}
		return simplePlan, meta, nil
	}
	if err != nil {
		return nil, meta, err
	}

	configs := []*referenceframe.LinearInputs{}
	if tgResult != nil {
		logger.CInfof(ctx, "traj-gen service returned %d samples (accelerations present: %v)",
			len(tgResult.configurations), len(tgResult.accelerations) > 0)
		configs = tgResult.configurations
	} else {
		logger.CInfof(ctx, "traj-gen service indicated arm is already at goal, skipping trajectory")
	}

	simplePlan, err := motionplan.NewSimplePlanFromTrajectory(configs, request.FrameSystem)
	if err != nil {
		return nil, meta, err
	}

	t := &TrajGenPlan{
		SimplePlan: simplePlan,
	}
	if tgResult != nil {
		t.Configurations = tgResult.configurations
		t.Velocities = tgResult.velocities
		t.Accelerations = tgResult.accelerations
		t.SampleTimes = tgResult.sampleTimes
	}

	if err := CheckPlanFromRequest(ctx, logger, request, t); err != nil {
		return nil, meta, err
	}

	return t, meta, nil
}
