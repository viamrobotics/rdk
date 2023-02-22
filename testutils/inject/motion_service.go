package inject

import (
	"context"

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
		worldState *referenceframe.WorldState,
		extra map[string]interface{},
	) (bool, error)
	GetPoseFunc func(
		ctx context.Context,
		componentName resource.Name,
		destinationFrame string,
		supplementalTransforms []*referenceframe.LinkInFrame,
		extra map[string]interface{},
	) (*referenceframe.PoseInFrame, error)
	DoCommandFunc func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
}

// Move calls the injected Move or the real variant.
func (mgs *MotionService) Move(
	ctx context.Context,
	componentName resource.Name,
	grabPose *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) (bool, error) {
	if mgs.MoveFunc == nil {
		return mgs.Service.Move(ctx, componentName, grabPose, worldState, extra)
	}
	return mgs.MoveFunc(ctx, componentName, grabPose, worldState, extra)
}

// MoveSingleComponent calls the injected MoveSingleComponent or the real variant. It uses the same function as Move.
func (mgs *MotionService) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	grabPose *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) (bool, error) {
	if mgs.MoveFunc == nil {
		return mgs.Service.MoveSingleComponent(ctx, componentName, grabPose, worldState, extra)
	}
	return mgs.MoveFunc(ctx, componentName, grabPose, worldState, extra)
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

// DoCommand calls the injected DoCommand or the real variant.
func (mgs *MotionService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if mgs.DoCommandFunc == nil {
		return mgs.Service.DoCommand(ctx, cmd)
	}
	return mgs.DoCommandFunc(ctx, cmd)
}
