package inject

import (
	"context"

	"github.com/golang/geo/r3"

	"go.viam.com/core/services/objectmanipulation"
)

// ObjectManipulationService represents a fake instance of an object manipulation
// service
type ObjectManipulationService struct {
	objectmanipulation.Service
	DoGrabFunc func(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error)
}

// DoGrab calls the injected DoGrab or the real variant
func (mgs *ObjectManipulationService) DoGrab(
	ctx context.Context,
	gripperName, armName, cameraName string,
	cameraPoint *r3.Vector,
) (bool, error) {
	if mgs.DoGrabFunc == nil {
		return mgs.Service.DoGrab(ctx, gripperName, armName, cameraName, cameraPoint)
	}
	return mgs.DoGrabFunc(ctx, gripperName, armName, cameraName, cameraPoint)
}
