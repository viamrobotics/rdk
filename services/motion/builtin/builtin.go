// Package builtin implements a motion service.
package builtin

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	servicepb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterDefaultService(
		motion.API,
		resource.DefaultServiceModel,
		resource.Registration[motion.Service, *Config]{
			Constructor: NewBuiltIn,
			WeakDependencies: []internal.ResourceMatcher{
				internal.SLAMDependencyWildcardMatcher,
				internal.ComponentDependencyWildcardMatcher,
			},
		})
}

const (
	builtinOpLabel      = "motion-service"
	maxTravelDistanceMM = 5e6 // this is equivalent to 5km
)

// inputEnabledActuator is an actuator that interacts with the frame system.
// This allows us to figure out where the actuator currently is and then
// move it. Input units are always in meters or radians.
type inputEnabledActuator interface {
	resource.Actuator
	referenceframe.InputEnabled
}

// ErrNotImplemented is thrown when an unreleased function is called.
var ErrNotImplemented = errors.New("function coming soon but not yet implemented")

// Config describes how to configure the service; currently only used for specifying dependency on framesystem service.
type Config struct {
	LogFilePath string `json:"log_file_path"`
}

// Validate here adds a dependency on the internal framesystem service.
func (c *Config) Validate(path string) ([]string, error) {
	return []string{framesystem.InternalServiceName.String()}, nil
}

// NewBuiltIn returns a new move and grab service for the given robot.
func NewBuiltIn(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (motion.Service, error) {
	ms := &builtIn{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := ms.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return ms, nil
}

func newFilePathLoggerConfig(filepath string) zap.Config {
	return zap.Config{
		Level:    zap.NewAtomicLevelAt(zap.DebugLevel),
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		DisableStacktrace: true,
		OutputPaths:       []string{filepath, "stdout"},
		ErrorOutputPaths:  []string{filepath, "stderr"},
	}
}

// Reconfigure updates the motion service when the config has changed.
func (ms *builtIn) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) (err error) {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	config, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	if config.LogFilePath != "" {
		logger, err := newFilePathLoggerConfig(config.LogFilePath).Build()
		if err != nil {
			return err
		}
		ms.logger = logger.Sugar().Named("motion")
	}
	movementSensors := make(map[resource.Name]movementsensor.MovementSensor)
	slamServices := make(map[resource.Name]slam.Service)
	components := make(map[resource.Name]resource.Resource)
	for name, dep := range deps {
		switch dep := dep.(type) {
		case framesystem.Service:
			ms.fsService = dep
		case movementsensor.MovementSensor:
			movementSensors[name] = dep
		case slam.Service:
			slamServices[name] = dep
		default:
			components[name] = dep
		}
	}
	ms.movementSensors = movementSensors
	ms.slamServices = slamServices
	ms.components = components
	return nil
}

type builtIn struct {
	resource.Named
	resource.TriviallyCloseable
	fsService       framesystem.Service
	movementSensors map[resource.Name]movementsensor.MovementSensor
	slamServices    map[resource.Name]slam.Service
	components      map[resource.Name]resource.Resource
	logger          golog.Logger
	lock            sync.Mutex
}

// Move takes a goal location and will plan and execute a movement to move a component specified by its name to that destination.
func (ms *builtIn) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	constraints *servicepb.Constraints,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	// get goal frame
	goalFrameName := destination.Parent()
	ms.logger.Debugf("goal given in frame of %q", goalFrameName)

	frameSys, err := ms.fsService.FrameSystem(ctx, worldState.Transforms())
	if err != nil {
		return false, err
	}

	// build maps of relevant components and inputs from initial inputs
	fsInputs, resources, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		return false, err
	}

	movingFrame := frameSys.Frame(componentName.ShortName())

	ms.logger.Debugf("frame system inputs: %v", fsInputs)
	if movingFrame == nil {
		return false, fmt.Errorf("component named %s not found in robot frame system", componentName.ShortName())
	}

	// re-evaluate goalPose to be in the frame of World
	solvingFrame := referenceframe.World // TODO(erh): this should really be the parent of rootName
	tf, err := frameSys.Transform(fsInputs, destination, solvingFrame)
	if err != nil {
		return false, err
	}
	goalPose, _ := tf.(*referenceframe.PoseInFrame)

	// the goal is to move the component to goalPose which is specified in coordinates of goalFrameName
	steps, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             ms.logger,
		Goal:               goalPose,
		Frame:              movingFrame,
		StartConfiguration: fsInputs,
		FrameSystem:        frameSys,
		WorldState:         worldState,
		ConstraintSpecs:    constraints,
		Options:            extra,
	})
	if err != nil {
		return false, err
	}

	// move all the components
	for _, step := range steps {
		// TODO(erh): what order? parallel?
		for name, inputs := range step {
			if len(inputs) == 0 {
				continue
			}
			r := resources[name]
			if err := r.GoToInputs(ctx, inputs); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if actuator, ok := r.(inputEnabledActuator); ok {
					if stopErr := actuator.Stop(ctx, nil); stopErr != nil {
						return false, errors.Wrap(err, stopErr.Error())
					}
				}
				return false, err
			}
		}
	}
	return true, nil
}

// MoveOnMap will move the given component to the given destination on the slam map generated from a slam service specified by slamName.
// Bases are the only component that supports this.
func (ms *builtIn) MoveOnMap(
	ctx context.Context,
	componentName resource.Name,
	destination spatialmath.Pose,
	slamName resource.Name,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()

	// make call to motionplan
	plan, kb, err := ms.planMoveOnMap(ctx, componentName, destination, slamName, kinematicsOptions, extra)
	if err != nil {
		return false, fmt.Errorf("error making plan for MoveOnMap: %w", err)
	}

	// execute the plan
	for i := 1; i < len(plan); i++ {
		if inputEnabledKb, ok := kb.(inputEnabledActuator); ok {
			if err := inputEnabledKb.GoToInputs(ctx, plan[i]); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if stopErr := kb.Stop(ctx, nil); stopErr != nil {
					return false, errors.Wrap(err, stopErr.Error())
				}
				return false, err
			}
		}
	}
	return true, nil
}

// MoveOnGlobe will move the given component to the given destination on the globe.
// Bases are the only component that supports this.
func (ms *builtIn) MoveOnGlobe(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	heading float64,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	motionCfg *motion.MotionConfiguration,
	extra map[string]interface{},
) (bool, error) {
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	// ensure arguments are well behaved
	if motionCfg == nil {
		motionCfg = &motion.MotionConfiguration{}
	}
	if obstacles == nil {
		obstacles = []*spatialmath.GeoObstacle{}
	}
	if destination == nil {
		return false, errors.New("destination cannot be nil")
	}

	moveRequest, err := ms.newMoveOnGlobeRequest(ctx, componentName, destination, movementSensorName, obstacles, motionCfg, extra)
	if err != nil {
		return false, err
	}

	// start a loop that plans every iteration and exits when something is read from the success channel
	for {
		ma := newMoveAttempt(ctx, moveRequest)
		if err := ma.start(); err != nil {
			return false, err
		}

		// this ensures that if the context is cancelled we always return early at the top of the loop
		if err := ctx.Err(); err != nil {
			ma.cancel()
			return false, err
		}

		select {
		// if context was cancelled by the calling function, error out
		case <-ctx.Done():
			ma.cancel()
			return false, ctx.Err()

		// once execution responds: return the result to the caller
		case resp := <-ma.responseChan:
			ms.logger.Debugf("execution complete: %#v", resp)
			ma.cancel()
			return resp.success, resp.err

		// if the position poller hit an error return it, otherwise replan
		case resp := <-ma.position.responseChan:
			ms.logger.Debugf("position response: %#v", resp)
			ma.cancel()
			if resp.err != nil {
				return false, resp.err
			}

		// if the obstacle poller hit an error return it, otherwise replan
		case resp := <-ma.obstacle.responseChan:
			ms.logger.Debugf("obstacle response: %#v", resp)
			ma.cancel()
			if resp.err != nil {
				return false, resp.err
			}
		}
	}
}

func (ms *builtIn) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	if destinationFrame == "" {
		destinationFrame = referenceframe.World
	}
	return ms.fsService.TransformPose(
		ctx,
		referenceframe.NewPoseInFrame(
			componentName.ShortName(),
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
		),
		destinationFrame,
		supplementalTransforms,
	)
}

// PlanMoveOnMap returns the plan for MoveOnMap to execute.
func (ms *builtIn) planMoveOnMap(
	ctx context.Context,
	componentName resource.Name,
	destination spatialmath.Pose,
	slamName resource.Name,
	kinematicsOptions kinematicbase.Options,
	extra map[string]interface{},
) ([][]referenceframe.Input, kinematicbase.KinematicBase, error) {
	// get the SLAM Service from the slamName
	slamSvc, ok := ms.slamServices[slamName]
	if !ok {
		return nil, nil, resource.DependencyNotFoundError(slamName)
	}

	// gets the extents of the SLAM map
	limits, err := slam.Limits(ctx, slamSvc)
	if err != nil {
		return nil, nil, err
	}
	limits = append(limits, referenceframe.Limit{Min: -2 * math.Pi, Max: 2 * math.Pi})

	// create a KinematicBase from the componentName
	component, ok := ms.components[componentName]
	if !ok {
		return nil, nil, resource.DependencyNotFoundError(componentName)
	}
	b, ok := component.(base.Base)
	if !ok {
		return nil, nil, fmt.Errorf("cannot move component of type %T because it is not a Base", component)
	}

	if extra != nil {
		if profile, ok := extra["motion_profile"]; ok {
			motionProfile, ok := profile.(string)
			if !ok {
				return nil, nil, errors.New("could not interpret motion_profile field as string")
			}
			kinematicsOptions.PositionOnlyMode = motionProfile == motionplan.PositionOnlyMotionProfile
		}
	}

	fs, err := ms.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, ms.logger, motion.NewSLAMLocalizer(slamSvc), limits, kinematicsOptions)
	if err != nil {
		return nil, nil, err
	}

	// replace original base frame with one that knows how to move itself and allow planning for
	if err = fs.ReplaceFrame(kb.Kinematics()); err != nil {
		return nil, nil, err
	}

	// get point cloud data in the form of bytes from pcd
	pointCloudData, err := slam.PointCloudMapFull(ctx, slamSvc)
	if err != nil {
		return nil, nil, err
	}
	// store slam point cloud data  in the form of a recursive octree for collision checking
	octree, err := pointcloud.ReadPCDToBasicOctree(bytes.NewReader(pointCloudData))
	if err != nil {
		return nil, nil, err
	}

	// get current position
	inputs, err := kb.CurrentInputs(ctx)
	if err != nil {
		return nil, nil, err
	}
	if kinematicsOptions.PositionOnlyMode && len(kb.Kinematics().DoF()) == 2 && len(inputs) == 3 {
		inputs = inputs[:2]
	}
	ms.logger.Debugf("base position: %v", inputs)

	dst := referenceframe.NewPoseInFrame(referenceframe.World, destination)

	f := kb.Kinematics()

	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{
		referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{octree}),
	}, nil)
	if err != nil {
		return nil, nil, err
	}

	seedMap := map[string][]referenceframe.Input{f.Name(): inputs}

	ms.logger.Debugf("goal position: %v", dst.Pose().Point())
	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             ms.logger,
		Goal:               dst,
		Frame:              f,
		StartConfiguration: seedMap,
		FrameSystem:        fs,
		WorldState:         worldState,
		ConstraintSpecs:    nil,
		Options:            extra,
	})
	if err != nil {
		return nil, nil, err
	}
	steps, err := plan.GetFrameSteps(f.Name())
	return steps, kb, err
}
