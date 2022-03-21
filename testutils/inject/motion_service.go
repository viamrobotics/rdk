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
	DoGrabFunc func(
		ctx context.Context,
		gripperName string,
		grabPose *referenceframe.PoseInFrame,
		obstacles []*referenceframe.GeometriesInFrame,
	) (bool, error)
}

// DoGrab calls the injected DoGrab or the real variant.
func (mgs *MotionService) DoGrab(
	ctx context.Context,
	gripperName string,
	grabPose *referenceframe.PoseInFrame,
	obstacles []*referenceframe.GeometriesInFrame,
) (bool, error) {
	if mgs.DoGrabFunc == nil {
		return mgs.Service.DoGrab(ctx, gripperName, grabPose, obstacles)
	}
	return mgs.DoGrabFunc(ctx, gripperName, grabPose, obstacles)
}
