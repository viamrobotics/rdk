package inject

import (
	"context"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
)

// MotionService represents a fake instance of an motion
// service.
type MotionService struct {
	motion.Service
	MoveFunc func(
		ctx context.Context,
		gripperName string,
		grabPose *referenceframe.PoseInFrame,
		obstacles []*referenceframe.GeometriesInFrame,
	) (bool, error)
}

// Move calls the injected Move or the real variant.
func (mgs *MotionService) Move(
	ctx context.Context,
	gripperName string,
	grabPose *referenceframe.PoseInFrame,
	obstacles []*referenceframe.GeometriesInFrame,
) (bool, error) {
	if mgs.MoveFunc == nil {
		return mgs.Service.Move(ctx, gripperName, grabPose, obstacles)
	}
	return mgs.MoveFunc(ctx, gripperName, grabPose, obstacles)
}
