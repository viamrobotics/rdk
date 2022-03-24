package inject

import (
	"context"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
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
	GetPoseFunc func(
		ctx context.Context,
		componentName resource.Name,
		destinationFrame string,
	) (*referenceframe.PoseInFrame, error)
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

// GetPose calls the injected GetPose or the real variant.
func (mgs *ObjectManipulationService) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
) (*referenceframe.PoseInFrame, error) {
	if mgs.GetPoseFunc == nil {
		return mgs.Service.GetPose(ctx, componentName, destinationFrame)
	}
	return mgs.GetPoseFunc(ctx, componentName, destinationFrame)
}
