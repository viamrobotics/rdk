package inject

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	servicepb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

// MotionService represents a fake instance of an motion
// service.
type MotionService struct {
	motion.Service
	name     resource.Name
	MoveFunc func(
		ctx context.Context,
		componentName resource.Name,
		grabPose *referenceframe.PoseInFrame,
		worldState *referenceframe.WorldState,
		constraints *servicepb.Constraints,
		extra map[string]interface{},
	) (bool, error)
	MoveOnMapFunc func(
		ctx context.Context,
		componentName resource.Name,
		destination spatialmath.Pose,
		slamName resource.Name,
		extra map[string]interface{},
	) (bool, error)
	MoveOnGlobeFunc func(
		ctx context.Context,
		componentName resource.Name,
		destination *geo.Point,
		heading float64,
		movementSensorName resource.Name,
		obstacles []*spatialmath.GeoObstacle,
		motionCfg *motion.MotionConfiguration,
		extra map[string]interface{},
	) (bool, error)
	MoveOnGlobeNewFunc func(
		ctx context.Context,
		req motion.MoveOnGlobeReq,
	) (string, error)
	GetPoseFunc func(
		ctx context.Context,
		componentName resource.Name,
		destinationFrame string,
		supplementalTransforms []*referenceframe.LinkInFrame,
		extra map[string]interface{},
	) (*referenceframe.PoseInFrame, error)
	StopPlanFunc func(
		ctx context.Context,
		rootComponent resource.Name,
		extra map[string]interface{},
	) error
	ListPlanStatusesFunc func(
		ctx context.Context,
		onlyActivePlans bool,
		extra map[string]interface{},
	) ([]motion.PlanStatusWithID, error)
	PlanHistoryFunc func(
		ctx context.Context,
		componentName resource.Name,
		lastPlanOnly bool,
		executionID string,
		extra map[string]interface{},
	) ([]motion.PlanWithStatus, error)
	DoCommandFunc func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func(ctx context.Context) error
}

// NewMotionService returns a new injected motion service.
func NewMotionService(name string) *MotionService {
	return &MotionService{name: motion.Named(name)}
}

// Name returns the name of the resource.
func (mgs *MotionService) Name() resource.Name {
	return mgs.name
}

// Move calls the injected Move or the real variant.
func (mgs *MotionService) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	constraints *servicepb.Constraints,
	extra map[string]interface{},
) (bool, error) {
	if mgs.MoveFunc == nil {
		return mgs.Service.Move(ctx, componentName, destination, worldState, constraints, extra)
	}
	return mgs.MoveFunc(ctx, componentName, destination, worldState, constraints, extra)
}

// MoveOnMap calls the injected MoveOnMap or the real variant.
func (mgs *MotionService) MoveOnMap(
	ctx context.Context,
	componentName resource.Name,
	destination spatialmath.Pose,
	slamName resource.Name,
	extra map[string]interface{},
) (bool, error) {
	if mgs.MoveOnMapFunc == nil {
		return mgs.Service.MoveOnMap(ctx, componentName, destination, slamName, extra)
	}
	return mgs.MoveOnMapFunc(ctx, componentName, destination, slamName, extra)
}

// MoveOnGlobe calls the injected MoveOnGlobe or the real variant.
func (mgs *MotionService) MoveOnGlobe(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	heading float64,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	motionCfg *motion.MotionConfiguration,
	extra map[string]interface{},
) (bool, error) {
	if mgs.MoveOnGlobeFunc == nil {
		return mgs.Service.MoveOnGlobe(ctx, componentName, destination, heading, movementSensorName, obstacles, motionCfg, extra)
	}
	return mgs.MoveOnGlobeFunc(ctx, componentName, destination, heading, movementSensorName, obstacles, motionCfg, extra)
}

// MoveOnGlobeNew calls the injected MoveOnGlobeNew or the real variant.
func (mgs *MotionService) MoveOnGlobeNew(ctx context.Context, req motion.MoveOnGlobeReq) (string, error) {
	if mgs.MoveOnGlobeNewFunc == nil {
		return mgs.Service.MoveOnGlobeNew(ctx, req)
	}
	return mgs.MoveOnGlobeNewFunc(ctx, req)
}

// GetPose calls the injected GetPose or the real variant.
func (mgs *MotionService) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	if mgs.GetPoseFunc == nil {
		return mgs.Service.GetPose(ctx, componentName, destinationFrame, supplementalTransforms, extra)
	}
	return mgs.GetPoseFunc(ctx, componentName, destinationFrame, supplementalTransforms, extra)
}

// StopPlan calls the injected StopPlan or the real variant.
func (mgs *MotionService) StopPlan(
	ctx context.Context,
	componentName resource.Name,
	extra map[string]interface{},
) error {
	if mgs.StopPlanFunc == nil {
		return mgs.Service.StopPlan(ctx, componentName, extra)
	}
	return mgs.StopPlanFunc(ctx, componentName, extra)
}

// ListPlanStatuses calls the injected ListPlanStatuses or the real variant.
func (mgs *MotionService) ListPlanStatuses(
	ctx context.Context,
	onlyActivePlans bool,
	extra map[string]interface{},
) ([]motion.PlanStatusWithID, error) {
	if mgs.ListPlanStatusesFunc == nil {
		return mgs.Service.ListPlanStatuses(ctx, onlyActivePlans, extra)
	}
	return mgs.ListPlanStatusesFunc(ctx, onlyActivePlans, extra)
}

// PlanHistory calls the injected PlanHistory or the real variant.
func (mgs *MotionService) PlanHistory(
	ctx context.Context,
	componentName resource.Name,
	lastPlanOnly bool,
	executionID string,
	extra map[string]interface{},
) ([]motion.PlanWithStatus, error) {
	if mgs.PlanHistoryFunc == nil {
		return mgs.Service.PlanHistory(ctx, componentName, lastPlanOnly, executionID, extra)
	}
	return mgs.PlanHistoryFunc(ctx, componentName, lastPlanOnly, executionID, extra)
}

// DoCommand calls the injected DoCommand or the real variant.
func (mgs *MotionService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if mgs.DoCommandFunc == nil {
		return mgs.Service.DoCommand(ctx, cmd)
	}
	return mgs.DoCommandFunc(ctx, cmd)
}

// Close calls the injected Close or the real version.
func (mgs *MotionService) Close(ctx context.Context) error {
	if mgs.CloseFunc == nil {
		if mgs.Service == nil {
			return nil
		}
		return mgs.Service.Close(ctx)
	}
	return mgs.CloseFunc(ctx)
}
