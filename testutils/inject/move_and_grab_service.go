package inject

import (
	"context"

	moveandgrab "go.viam.com/core/services/move_and_grab"
)

// MoveAndGrabService represents a fake instance of a move and grab
// service
type MoveAndGrabService struct {
	moveandgrab.Service
	DoGrabFunc func(ctx context.Context, cameraName string, x, y, z float64) (bool, error)
}

// DoGrab calls the injected DoGrab or the real variant
func (mgs *MoveAndGrabService) DoGrab(ctx context.Context, cameraName string, x, y, z float64) (bool, error) {
	if mgs.DoGrabFunc == nil {
		return mgs.Service.DoGrab(ctx, cameraName, x, y, z)
	}
	return mgs.DoGrabFunc(ctx, cameraName, x, y, z)
}
