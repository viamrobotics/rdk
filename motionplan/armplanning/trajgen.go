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
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
)

// TrajGenConfig holds configuration for the trajectory generator ML model service.
type TrajGenConfig struct {
	Service                            string   `json:"service"`
	PathToleranceDeltaRads             *float64 `json:"path_tolerance_delta_rads,omitempty"`
	PathColinearizationRatio           *float64 `json:"path_colinearization_ratio,omitempty"`
	WaypointDeduplicationToleranceRads *float64 `json:"waypoint_deduplication_tolerance_rads,omitempty"`
	VelocityLimitsRadsPerSec           float64  `json:"velocity_limits_rads_per_sec,omitempty"`
	AccelerationLimitsRadsPerSec2      float64  `json:"acceleration_limits_rads_per_sec2,omitempty"`
	SamplingFreqHz                     *float64 `json:"trajectory_sampling_freq_hz,omitempty"`
}

// Validate returns the mlmodel service name as a required dependency and checks that velocity and
// acceleration limits are positive.
func (cfg *TrajGenConfig) Validate(path string) ([]string, error) {
	if cfg.VelocityLimitsRadsPerSec <= 0 {
		return nil, fmt.Errorf("need positive velocity_limits_rads_per_sec if using trajectory_generator, got %v",
			cfg.VelocityLimitsRadsPerSec)
	}
	if cfg.AccelerationLimitsRadsPerSec2 <= 0 {
		return nil, fmt.Errorf("need positive acceleration_limits_rads_per_sec2 if using trajectory_generator, got %v",
			cfg.AccelerationLimitsRadsPerSec2)
	}
	if cfg.Service == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "service")
	}
	return []string{cfg.Service}, nil
}

// ToTrajGen resolves the named mlmodel service from deps and returns a TrajGen ready for use.
func (cfg *TrajGenConfig) ToTrajGen(deps resource.Dependencies) (*TrajGen, error) {
	svc, err := mlmodel.FromProvider(deps, cfg.Service)
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

// DoCommandKeyExecuteTrajGenPlan is the do_command key for sending a precomputed kinodynamic
// trajectory to an arm component that supports it.
const DoCommandKeyExecuteTrajGenPlan = "execute_traj_gen_plan"

// DoCommandKeySupportsExecuteTrajGenPlan is the capability probe key. Arms that support
// execute_traj_gen_plan respond to this with true.
const DoCommandKeySupportsExecuteTrajGenPlan = "supports_execute_traj_gen_plan"

// TrajGen holds a resolved trajectory generator ML model service along with its configuration.
type TrajGen struct {
	trajGen                            mlmodel.Service
	PathToleranceDeltaRads             float64 `json:"path_tolerance_delta_rads"`
	PathColinearizationRatio           float64 `json:"path_colinearization_ratio"`
	WaypointDeduplicationToleranceRads float64 `json:"waypoint_deduplication_tolerance_rads"`
	VelocityLimitsRadsPerSec           float64 `json:"velocity_limits_rads_per_sec"`
	AccelerationLimitsRadsPerSec2      float64 `json:"acceleration_limits_rads_per_sec2"`
	SamplingFreqHz                     float64 `json:"trajectory_sampling_freq_hz"`
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
	PathToleranceDeltaRads             *float64 `json:"path_tolerance_delta_rads,omitempty"`
	PathColinearizationRatio           *float64 `json:"path_colinearization_ratio,omitempty"`
	WaypointDeduplicationToleranceRads *float64 `json:"waypoint_deduplication_tolerance_rads,omitempty"`
	VelocityLimitsRadsPerSec           *float64 `json:"velocity_limits_rads_per_sec,omitempty"`
	AccelerationLimitsRadsPerSec2      *float64 `json:"acceleration_limits_rads_per_sec2,omitempty"`
	SamplingFreqHz                     *float64 `json:"trajectory_sampling_freq_hz,omitempty"`
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
		copy.VelocityLimitsRadsPerSec = *o.VelocityLimitsRadsPerSec
	}
	if o.AccelerationLimitsRadsPerSec2 != nil {
		copy.AccelerationLimitsRadsPerSec2 = *o.AccelerationLimitsRadsPerSec2
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
	velocityLimitsRadsPerSec float64,
	accelerationLimitsRadsPerSec2 float64,
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

// DoCommandPayload returns the map[string]any value for the "execute_traj_gen_plan" do_command key.
func (t *TrajGenPlan) DoCommandPayload() map[string]any {
	flatten := func(lis []*referenceframe.LinearInputs) [][]float64 {
		out := make([][]float64, len(lis))
		for i, li := range lis {
			out[i] = li.GetLinearizedInputs()
		}
		return out
	}
	payload := map[string]any{
		"configurations_rads":     flatten(t.Configurations),
		"velocities_rads_per_sec": flatten(t.Velocities),
		"sample_times_sec":        t.SampleTimes,
	}
	if len(t.Accelerations) > 0 {
		payload["accelerations_rads_per_sec2"] = flatten(t.Accelerations)
	}
	return payload
}

// trajGenResult is the raw output of inferTrajGen.
type trajGenResult struct {
	configurations []*referenceframe.LinearInputs
	velocities     []*referenceframe.LinearInputs
	accelerations  []*referenceframe.LinearInputs // nil when not provided by the service
	sampleTimes    []float64
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

	velLimits := make([]float64, dof)
	accelLimits := make([]float64, dof)
	for i := range dof {
		velLimits[i] = tg.VelocityLimitsRadsPerSec
		accelLimits[i] = tg.AccelerationLimitsRadsPerSec2
	}

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
			tensor.Of(tensor.Int64),
			tensor.WithShape(1),
			tensor.WithBacking([]int64{int64(tg.SamplingFreqHz)}),
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
