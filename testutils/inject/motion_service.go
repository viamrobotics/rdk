package inject

import (
	"context"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
)

// MotionService represents a fake instance of an motion
// service.
type MotionService struct {
	motion.Service
	MoveFunc func(
		ctx context.Context,
		componentName resource.Name,
		grabPose *referenceframe.PoseInFrame,
		worldState *commonpb.WorldState,
	) (bool, error)
	GetPoseFunc func(
		ctx context.Context,
		componentName resource.Name,
		destinationFrame string,
		supplementalTransforms []*commonpb.Transform,
	) (*referenceframe.PoseInFrame, error)
}

// PlanAndMove calls the injected Move or the real variant.
func (mgs *MotionService) PlanAndMove(
	ctx context.Context,
	componentName resource.Name,
	grabPose *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	if mgs.MoveFunc == nil {
		return mgs.Service.PlanAndMove(ctx, componentName, grabPose, worldState)
	}
	return mgs.MoveFunc(ctx, componentName, grabPose, worldState)
}

// MoveSingleComponent calls the injected MoveSingleComponent or the real variant. It uses the same function as PlanAndMove.
func (mgs *MotionService) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	grabPose *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	if mgs.MoveFunc == nil {
		return mgs.Service.MoveSingleComponent(ctx, componentName, grabPose, worldState)
	}
	return mgs.MoveFunc(ctx, componentName, grabPose, worldState)
}

// GetPose calls the injected GetPose or the real variant.
func (mgs *MotionService) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	if mgs.GetPoseFunc == nil {
		return mgs.Service.GetPose(ctx, componentName, destinationFrame, supplementalTransforms)
	}
	return mgs.GetPoseFunc(ctx, componentName, destinationFrame, supplementalTransforms)
}
