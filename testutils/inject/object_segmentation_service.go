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
	GetSegmentationFunc func(ctx context.Context, cameraName string, params *vision.Parameters3D) ([]*vision.Object, error)
}

// GetSegmentation calls the injected GetObjectPointClouds or the real variant.
func (seg *ObjectSegmentationService) GetSegmentation(
	ctx context.Context,
	cameraName string,
	params *vision.Parameters3D,
) ([]*vision.Object, error) {
	if seg.GetSegmentationFunc == nil {
		return seg.Service.GetSegmentation(ctx, cameraName, params)
	}
	return seg.GetSegmentationFunc(ctx, cameraName, params)
}
