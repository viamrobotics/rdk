package inject

import (
	"context"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/objectmanipulation"
)

// ObjectManipulationService represents a fake instance of an object manipulation
// service.
type ObjectManipulationService struct {
	objectmanipulation.Service
	DoGrabFunc func(
		ctx context.Context,
		gripperName string,
		grabPose *referenceframe.PoseInFrame,
		obstacles []*referenceframe.GeometriesInFrame,
	) (bool, error)
}

// DoGrab calls the injected DoGrab or the real variant.
func (mgs *ObjectManipulationService) DoGrab(
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
