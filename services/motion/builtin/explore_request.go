package builtin

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
)

const (
	moveLimitMM             = 100000.
	validObstacleDistanceMM = 1000.
)

type exploreRequest struct {
	config        *motion.MotionConfiguration
	plan          motionplan.Plan
	kinematicBase kinematicbase.KinematicBase
	camera        camera.Camera
	visionService vision.Service
}

func (ms *builtIn) newMoveExploreRequest(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	motionCfg *motion.MotionConfiguration,
	seedPlan motionplan.Plan,
	extra map[string]interface{},
) (*exploreRequest, error) {

	// Create kinematic base
	kb, err := ms.createKinematicBase(ctx, componentName, extra)
	if err != nil {
		return nil, err
	}

	// Create motionplan plan
	planInputs, err := ms.createMotionPlan(ctx, kb, destination.Pose(), true, extra)
	if err != nil {
		return nil, err
	}

	var plan motionplan.Plan
	for _, inputs := range planInputs {
		input := make(map[string][]referenceframe.Input)
		input[kb.Name().Name] = inputs
		plan = append(plan, input)
	}

	return &exploreRequest{
		config:        motionCfg,
		kinematicBase: kb,
		plan:          plan,
		camera:        ms.camera,
		visionService: ms.visionService,
	}, nil
}

func createKBOps(extra map[string]interface{}) (kinematicbase.Options, error) {
	opt := kinematicbase.NewKinematicBaseOptions()
	opt.NoSkidSteer = true
	opt.UsePTGs = false

	extra["motion_profile"] = motionplan.PositionOnlyMotionProfile

	if degsPerSec, ok := extra["angular_degs_per_sec"]; ok {
		angularDegsPerSec, ok := degsPerSec.(float64)
		if !ok {
			return kinematicbase.Options{}, errors.New("could not interpret motion_profile field as string")
		}
		opt.AngularVelocityDegsPerSec = angularDegsPerSec
	}

	if mPerSec, ok := extra["linear_m_per_sec"]; ok {
		linearMPerSec, ok := mPerSec.(float64)
		if !ok {
			return kinematicbase.Options{}, errors.New("could not interpret motion_profile field as string")
		}
		opt.LinearVelocityMMPerSec = linearMPerSec
	}

	if profile, ok := extra["motion_profile"]; ok {
		motionProfile, ok := profile.(string)
		if !ok {
			return kinematicbase.Options{}, errors.New("could not interpret motion_profile field as string")
		}
		opt.PositionOnlyMode = motionProfile == motionplan.PositionOnlyMotionProfile
	}

	return opt, nil
}

// PlanMoveOnMap returns the plan for MoveOnMap to execute.
func (ms *builtIn) createKinematicBase(
	ctx context.Context,
	componentName resource.Name,
	extra map[string]interface{},
) (kinematicbase.KinematicBase, error) {
	// create a KinematicBase from the componentName
	component, ok := ms.components[componentName]
	if !ok {
		return nil, resource.DependencyNotFoundError(componentName)
	}

	b, ok := component.(base.Base)
	if !ok {
		return nil, fmt.Errorf("cannot move component of type %T because it is not a Base", component)
	}

	kinematicsOptions, err := createKBOps(extra)
	if err != nil {
		return nil, err
	}

	kb, err := kinematicbase.WrapWithKinematics(
		ctx,
		b,
		ms.logger,
		nil,
		[]referenceframe.Limit{{Min: -moveLimitMM, Max: moveLimitMM}, {Min: -moveLimitMM, Max: moveLimitMM}},
		kinematicsOptions,
	)
	if err != nil {
		return nil, err
	}

	return kb, nil
}

func (ms *builtIn) createMotionPlan(
	ctx context.Context,
	kb kinematicbase.KinematicBase,
	destination spatialmath.Pose,
	positionOnlyMode bool,
	extra map[string]interface{},
) ([][]referenceframe.Input, error) {
	fs, err := ms.fsService.FrameSystem(ctx, nil)
	if err != nil {
		return nil, err
	}

	// replace original base frame with one that knows how to move itself and allow planning for
	if err := fs.ReplaceFrame(kb.Kinematics()); err != nil {
		return nil, err
	}

	inputs := []referenceframe.Input{{Value: 0}, {Value: 0}, {Value: 0}}

	if positionOnlyMode && len(kb.Kinematics().DoF()) == 2 && len(inputs) == 3 {
		inputs = inputs[:2]
	}

	dst := referenceframe.NewPoseInFrame(referenceframe.World, destination)

	f := kb.Kinematics()

	worldStateNew, err := referenceframe.NewWorldState(nil, nil)
	if err != nil {
		return nil, err
	}

	seedMap := map[string][]referenceframe.Input{f.Name(): inputs}

	ms.logger.Debugf("goal position: %v", dst.Pose().Point())
	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             ms.logger,
		Goal:               dst,
		Frame:              f,
		StartConfiguration: seedMap,
		FrameSystem:        fs,
		WorldState:         worldStateNew,
		ConstraintSpecs:    nil,
		Options:            extra,
	})
	if err != nil {
		return nil, err
	}
	steps, err := plan.GetFrameSteps(f.Name())
	return steps, err
}
