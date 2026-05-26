// Package sim implements the arm API and simulates moving to joint positions over time. It offers
// an API to do so in a completely deterministic manner for testing.
package sim

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// operation represents an in-flight TOTG-planned trajectory.
//
// Logical states / invariants:
// 1. Default constructed -- no operation in flight (sampleTimes == nil)
// 2. Operation started -> sampleTimes != nil, done == false, stopped == false
// 3. Operation succeeded -> done == true
// 4. Operation stopped -> stopped == true; sampleTimes cleared
type operation struct {
	// sampleConfigs is row-major [n_samples, n_dof]; sampleTimes is [n_samples] in seconds
	// from t=0. Both nil iff no operation is in flight.
	sampleTimes   []float64
	sampleConfigs []float64
	nDof          int

	// trajStart anchors sampleTimes[0]==0 to the simulation clock (sa.lastUpdated).
	trajStart time.Time

	done    bool
	stopped bool
}

func (op operation) isMoving() bool {
	return op.sampleTimes != nil && !op.done && !op.stopped
}

type simulatedArm struct {
	resource.Named

	// logical properties
	modelName string
	model     referenceframe.Model

	// speed is the per-joint velocity limit in radians per second, applied uniformly across
	// all joints by the TOTG planner.
	speed float64
	// acceleration is the per-joint acceleration limit in radians per second squared,
	// applied uniformly across all joints by the TOTG planner.
	acceleration float64
	// pathTolerance is the corner-blend tolerance in radians passed to the TOTG planner.
	// Zero means the trajectory passes exactly through every interior waypoint
	// (decelerating to zero at each corner); positive values allow blending.
	pathTolerance float64

	// lifetime management
	resource.AlwaysRebuild
	closed atomic.Bool
	ctx    context.Context
	cancel func()

	// operational properties
	mu sync.Mutex
	// `currInputs` is always atomically updated along with `lastUpdated`. This is in radians.
	currInputs []float64
	// `lastUpdated` can be assumed to be initialized to the zero value when not simulating time.
	lastUpdated time.Time
	operation   operation

	// timeSimulation manages a background goroutine that updates time and the arm's position (via
	// `updateForTime`) every few milliseconds. It is off by default. Time simulation can be turned
	// on with the `Config.SimulateTime` option.
	//
	// When off, the owner of the `simulatedArm` must explicitly call `updateForTime` for the arm to
	// "move".
	timeSimulation *utils.StoppableWorkers

	logger logging.Logger

	gen trajectoryGenerator
}

// Config is used for converting config attributes.
type Config struct {
	Model         string `json:"arm-model,omitempty"`
	ModelFilePath string `json:"model-path,omitempty"`

	// Speed represents how quickly the joints of the arm will move. Speed is in radians per
	// second. In this simplified simulation, speed is the same for each joint.
	Speed float64 `json:"speed,omitempty"`

	// Acceleration is the per-joint acceleration limit in radians per second squared, applied
	// uniformly across all joints. Zero or negative selects the default of Speed*10, giving
	// roughly 0.1 s to reach cruise velocity.
	Acceleration float64 `json:"acceleration,omitempty"`

	// PathTolerance is the corner-blend tolerance in radians passed to the TOTG planner.
	// Zero (the default) means the trajectory passes exactly through every interior
	// waypoint. Positive values allow smooth blending through corners.
	PathTolerance float64 `json:"path-tolerance,omitempty"`

	// SimulateTime controls whether the `simulatedArm` will spin up and manage a background
	// goroutine for continually updating time to a real-world value.
	SimulateTime bool `json:"simulate-time,omitempty"`
}

func init() {
	resource.RegisterComponent(arm.API, Model, resource.Registration[arm.Arm, *Config]{
		Constructor: NewArm,
	})
}

// Model is the name used to refer to the simulated arm model.
var Model = resource.DefaultModelFamily.WithModel("simulated")

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	var err error
	switch {
	case conf.Model != "" && conf.ModelFilePath != "":
		return nil, nil, errors.New("can only populate either ArmModel or ModelPath - not both")
	case conf.Model != "" && conf.ModelFilePath == "":
		_, err = modelFromName(conf.Model, "")
	case conf.Model == "" && conf.ModelFilePath != "":
		_, err = referenceframe.KinematicModelFromFile(conf.ModelFilePath, "")
	}
	return nil, nil, err
}

// NewArm is the `func init` registered constructor intended to be consumed/invoked by the resource
// graph.
func NewArm(ctx context.Context, deps resource.Dependencies, resConf resource.Config, logger logging.Logger,
) (arm.Arm, error) {
	armConf, err := resource.NativeConfig[*Config](resConf)
	if err != nil {
		return nil, err
	}

	speed := 1.0 // 1 radian per second
	if armConf.Speed > 0.0 {
		speed = armConf.Speed
	}

	// Default acceleration reaches cruise in ~0.1 s.
	acceleration := speed * 10.0
	if armConf.Acceleration > 0.0 {
		acceleration = armConf.Acceleration
	}

	model, err := buildModel(resConf.Name, armConf)
	if err != nil {
		return nil, err
	}

	return newArm(
		resConf.ResourceName().AsNamed(), armConf.Model, model,
		speed, acceleration, armConf.PathTolerance, armConf.SimulateTime,
		logger,
	), nil
}

func newArm(
	namedArm resource.Named, modelName string, model referenceframe.Model,
	speed, acceleration, pathTolerance float64,
	simulateTime bool,
	logger logging.Logger,
) *simulatedArm {
	ctx, cancel := context.WithCancel(context.Background())
	ret := &simulatedArm{
		Named:  namedArm,
		logger: logger,

		modelName:     modelName,
		model:         model,
		speed:         speed,
		acceleration:  acceleration,
		pathTolerance: pathTolerance,

		ctx:    ctx,
		cancel: cancel,

		currInputs: make([]float64, len(model.DoF())),
	}

	if simulateTime {
		// When simulating time, avoid ever letting the zero value be visible. Lest the first
		// movement be unpredictable.
		ret.lastUpdated = time.Now()
		ret.timeSimulation = utils.NewStoppableWorkerWithTicker(10*time.Millisecond, func(_ context.Context) {
			ret.updateForTime(time.Now())
		})
	}

	ret.gen = newTrajectoryGenerator(logger)

	return ret
}

// Simulated arms only update their position when `updateForTime` is called. This can be used by
// tests for deterministic passage of time. Or can be called by a background goroutine to follow a
// realtime clock.
//
// But direct `arm.Arm` API calls will _not_ update the position under the hood.
func (sa *simulatedArm) updateForTime(now time.Time) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if !sa.operation.isMoving() {
		sa.lastUpdated = now
		return
	}

	sa.lastUpdated = now

	op := &sa.operation
	last := len(op.sampleTimes) - 1
	if last < 0 {
		// Shouldn't happen given the isMoving guard, but be defensive.
		op.done = true
		return
	}

	t := now.Sub(op.trajStart).Seconds()

	// Declare done when within one sample period of the trajectory end. The trajex sampler
	// in uniform_sampler.cpp spaces samples evenly from 0 to `duration`, and `duration` is
	// unavoidably larger than the nominal motion time by the acceleration ramp. One sample
	// period of slop matches the sampler's own quantization granularity.
	if last == 0 || t >= op.sampleTimes[last-1] {
		copy(sa.currInputs, op.sampleConfigs[last*op.nDof:(last+1)*op.nDof])
		op.done = true
		return
	}

	// Samples are uniformly spaced (sampleTimes[k] == k*dt).
	dt := op.sampleTimes[last] / float64(last)
	k := int(t / dt)
	if k < 0 {
		k = 0
	}
	alpha := (t - op.sampleTimes[k]) / dt
	if alpha < 0 {
		alpha = 0
	}
	for j := 0; j < op.nDof; j++ {
		a := op.sampleConfigs[k*op.nDof+j]
		b := op.sampleConfigs[(k+1)*op.nDof+j]
		sa.currInputs[j] = a + alpha*(b-a)
	}
}

func (sa *simulatedArm) EndPosition(
	ctx context.Context, extra map[string]interface{},
) (spatialmath.Pose, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	return sa.model.Transform(sa.currInputs)
}

func (sa *simulatedArm) MoveToPosition(
	ctx context.Context, pose spatialmath.Pose, extra map[string]interface{},
) error {
	return armplanning.MoveArm(ctx, sa.logger, sa, pose)
}

func (sa *simulatedArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return sa.JointPositions(ctx, nil)
}

func (sa *simulatedArm) JointPositions(
	ctx context.Context, extra map[string]interface{},
) ([]referenceframe.Input, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	ret := make([]referenceframe.Input, len(sa.currInputs))
	copy(ret, sa.currInputs)

	return ret, nil
}

func (sa *simulatedArm) MoveToJointPositions(
	ctx context.Context, target []referenceframe.Input, _ map[string]interface{},
) error {
	return sa.MoveThroughJointPositions(ctx, [][]referenceframe.Input{target}, nil, nil)
}

func (sa *simulatedArm) MoveThroughJointPositions(
	ctx context.Context,
	positions [][]referenceframe.Input,
	_ *arm.MoveOptions,
	_ map[string]interface{},
) error {
	if len(positions) == 0 {
		return nil
	}

	for _, wp := range positions {
		if err := arm.CheckDesiredJointPositions(ctx, sa, wp); err != nil {
			return err
		}
	}

	sa.mu.Lock()
	if sa.operation.isMoving() {
		sa.mu.Unlock()
		return errors.New("arm is already moving")
	}

	// Build the waypoint list with the current pose prepended. Dedup before
	// planning so the "already at target" case short-circuits cleanly without
	// involving the trajectory generator.
	waypoints := make([][]float64, 0, len(positions)+1)
	waypoints = append(waypoints, sa.currInputs)
	waypoints = append(waypoints, positions...)
	waypoints = dedupWaypoints(waypoints, defaultDedupToleranceRads)
	if len(waypoints) < 2 {
		sa.mu.Unlock()
		return nil
	}

	traj, err := sa.gen.Plan(ctx, waypoints, sa.speed, sa.acceleration, sa.pathTolerance)
	if err != nil {
		sa.mu.Unlock()
		return err
	}

	sa.operation = operation{
		sampleTimes:   traj.sampleTimes,
		sampleConfigs: traj.sampleConfigs,
		nDof:          traj.nDof,
		trajStart:     sa.lastUpdated,
	}
	sa.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sa.ctx.Done():
			return sa.ctx.Err()
		default:
			sa.mu.Lock()
			done, stopped := sa.operation.done, sa.operation.stopped
			sa.mu.Unlock()

			if done && stopped {
				panic("cannot be both done and stopped")
			}
			if done {
				return nil
			}
			if stopped {
				return errors.New("stopped before reaching target")
			}
			time.Sleep(time.Millisecond)
		}
	}
}

func (sa *simulatedArm) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	return sa.MoveThroughJointPositions(ctx, inputSteps, nil, nil)
}

func (sa *simulatedArm) IsMoving(ctx context.Context) (bool, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	return sa.operation.isMoving(), nil
}

func (sa *simulatedArm) Stop(ctx context.Context, extra map[string]any) error {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	// Only set `stopped` if we are moving. Otherwise the information that distinguishes whether the
	// arm stopped moving because it reached the goal, or because it was stopped is lost.
	if !sa.operation.isMoving() {
		return nil
	}

	sa.operation.stopped = true
	sa.operation.sampleTimes = nil
	sa.operation.sampleConfigs = nil

	return nil
}

func (sa *simulatedArm) Close(ctx context.Context) error {
	sa.closed.Store(true)
	sa.cancel()
	if sa.timeSimulation != nil {
		sa.timeSimulation.Stop()
	}

	return nil
}
