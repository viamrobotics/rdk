// Package sim implements the arm API and simulates moving to joint positions over time. It offers
// an API to do so in a completely deterministic manner for testing.
package sim

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// operation has the following logical states/invariants:
// 1. Default constructed -- no operation in flight
// 2. Operation started -> targetInputs != nil, done == false, stopped == false
// 3. Operation successful -> done == true
// 4. Operation failed -> stopped == true
type operation struct {
	targetInputs []float64
	done         bool
	stopped      bool
}

func (op operation) isMoving() bool {
	// If we have `targetInputs` and we are not done and not stopped
	return op.targetInputs != nil && !op.done && !op.stopped
}

type simulatedArm struct {
	resource.Named

	// logical properties
	modelName string
	model     referenceframe.Model
	// Speed represents how quickly the joints of the arm will move. Speed is in radians per
	// second. In this simplified simulation, speed is the same for each joint.
	speed float64

	// lifetime management
	resource.AlwaysRebuild
	closed atomic.Bool
	ctx    context.Context
	cancel func()

	// operational properties
	mu sync.Mutex
	// `currInputs` is always atomically updated along with `lastUpdated`.
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
}

// Config is used for converting config attributes.
type Config struct {
	Model         string `json:"arm-model,omitempty"`
	ModelFilePath string `json:"model-path,omitempty"`

	// Speed represents how quickly the joints of the arm will move. Speed is in radians per
	// second. In this simplified simulation, speed is the same for each joint.
	Speed float64 `json:"speed,omitempty"`

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

	model, err := buildModel(resConf.Name, armConf)
	if err != nil {
		return nil, err
	}

	return newArm(resConf.ResourceName().AsNamed(), armConf.Model, model, speed, armConf.SimulateTime, logger), nil
}

func newArm(
	namedArm resource.Named, modelName string, model referenceframe.Model, speed float64, simulateTime bool,
	logger logging.Logger,
) *simulatedArm {
	ctx, cancel := context.WithCancel(context.Background())
	ret := &simulatedArm{
		Named:  namedArm,
		logger: logger,

		modelName: modelName,
		model:     model,
		speed:     speed,

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

	// This will track if we need to set `done` to true. So long as even one joint is moving, this
	// operation is not "done".
	anyJointStillMoving := false

	// Dan: There are three natural algorithms for simulating an arm in motion:
	// - Each joint moves at its maximum speed. Joints will finish moving at different times.
	// - Joints that have less travel will move slower such that all joints finish at the same time.
	// - Joints move such that the end effector follows a straight-line path from start to finish.
	//   - It's not obvious to me there is always a valid straight-line path. E.g: avoiding
	//     self-collisions.
	//
	// I perceive its easiest to implement the first algorithm. That's what the following code
	// implements. Though I expect the second or third better model arms in the wild. No harm in
	// implementing other algorithms and adding a configuration knob to switch between them.
	timeSinceLastUpdate := now.Sub(sa.lastUpdated)
	sa.lastUpdated = now
	for jointIdx, currJointInp := range sa.currInputs {
		// The remaining distance between our target and where we are. Note the result is signed
		// based on the direction we are traveling.
		diffRads := sa.operation.targetInputs[jointIdx] - sa.currInputs[jointIdx]

		// How far we can theoretically travel since the last update. We will "cap" this to
		// `diffRads`.
		toTravelRads := timeSinceLastUpdate.Seconds() * sa.speed
		const epsilon = 1e-9
		if toTravelRads > math.Abs(diffRads)-epsilon {
			// We can travel farther than we need to. Simply set the current joint position to its
			// target.
			sa.currInputs[jointIdx] = sa.operation.targetInputs[jointIdx]
		} else {
			// We have not reached our destination. Advance the current joint position by
			// `toTravelRads`.
			if diffRads < 0 {
				// `toTravelRads` is always positive, flip the sign if we need to travel in the
				// opposite direction.
				toTravelRads = -toTravelRads
			}

			sa.currInputs[jointIdx] = currJointInp + toTravelRads
			anyJointStillMoving = true
		}
	}

	if !anyJointStillMoving {
		// All joints hit the "traveling further than we need to" path. The operation is done.
		sa.operation.done = true
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
	return errors.New("unimplemented -- must call with explicit joint positions")
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

func (sa *simulatedArm) StreamJointPositions(
	ctx context.Context, fps int32, extra map[string]interface{},
) (chan *arm.JointPositionsStreamed, error) {
	if fps <= 0 {
		fps = 30
	}

	ch := make(chan *arm.JointPositionsStreamed, 8)
	ticker := time.NewTicker(time.Second / time.Duration(fps))

	go func() {
		defer close(ch)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				positions, err := sa.JointPositions(ctx, extra)
				if err != nil {
					return
				}

				ch <- &arm.JointPositionsStreamed{
					Positions: positions,
					Timestamp: time.Now(),
				}
			}
		}
	}()

	return ch, nil
}

func (sa *simulatedArm) MoveToJointPositions(
	ctx context.Context, target []referenceframe.Input, extra map[string]interface{},
) error {
	if err := arm.CheckDesiredJointPositions(ctx, sa, target); err != nil {
		return err
	}

	sa.mu.Lock()
	sa.operation = operation{
		targetInputs: target,
		done:         false,
		stopped:      false,
	}
	sa.mu.Unlock()

	// An operation was "started". `MoveToJointPositions` blocks until the movement completes or is
	// canceled.
	for {
		select {
		case <-ctx.Done():
			// Command cancelation.
			return ctx.Err()

		case <-sa.ctx.Done():
			// `simulatedArm.Close` was called or robot shutdown.
			return sa.ctx.Err()

		default:
			// Poll for completion:
			sa.mu.Lock()
			// Calls to `updateForTime` will nil out `targetInputs` when a movement is completed.
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

func (sa *simulatedArm) MoveThroughJointPositions(
	ctx context.Context,
	positions [][]referenceframe.Input,
	_ *arm.MoveOptions,
	_ map[string]interface{},
) error {
	for _, goal := range positions {
		if err := sa.MoveToJointPositions(ctx, goal, nil); err != nil {
			return err
		}
	}

	return nil
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
