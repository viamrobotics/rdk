package inject

import (
	"context"

	"go.viam.com/core/services/objectmanipulation"
)

// ObjectManipulationService represents a fake instance of an object manipulation
// service
type ObjectManipulationService struct {
	objectmanipulation.Service
	DoGrabFunc func(ctx context.Context, cameraName string, x, y, z float64) (bool, error)
}

// DoGrab calls the injected DoGrab or the real variant
func (mgs *ObjectManipulationService) DoGrab(ctx context.Context, gripperName, armName, cameraName string, x, y, z float64) (bool, error) {
	if mgs.DoGrabFunc == nil {
		return mgs.Service.DoGrab(ctx, gripperName, armName, cameraName, x, y, z)
	}
	return mgs.DoGrabFunc(ctx, cameraName, x, y, z)
}
