package inject

import (
	"context"

	"go.viam.com/rdk/services/objectsegmentation"
	"go.viam.com/rdk/vision"
)

// ObjectSegmentationService represents a fake instance of an object segmentation
// service.
type ObjectSegmentationService struct {
	objectsegmentation.Service
	GetObjectsFunc func(ctx context.Context, cameraName string, params *vision.Parameters3D) ([]*vision.Object, error)
}

// GetObjects calls the injected GetObjects or the real variant.
func (seg *ObjectSegmentationService) GetObjects(
	ctx context.Context,
	cameraName string,
	params *vision.Parameters3D,
) ([]*vision.Object, error) {
	if seg.GetObjectsFunc == nil {
		return seg.Service.GetObjects(ctx, cameraName, params)
	}
	return seg.GetObjectsFunc(ctx, cameraName, params)
}
