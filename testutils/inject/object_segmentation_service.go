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
	GetObjectPointCloudsFunc func(ctx context.Context, cameraName string, params *vision.Parameters3D) ([]*vision.Object, error)
}

// GetObjectPointClouds calls the injected GetObjectPointClouds or the real variant.
func (seg *ObjectSegmentationService) GetObjectPointClouds(
	ctx context.Context,
	cameraName string,
	params *vision.Parameters3D,
) ([]*vision.Object, error) {
	if seg.GetObjectPointCloudsFunc == nil {
		return seg.Service.GetObjectPointClouds(ctx, cameraName, params)
	}
	return seg.GetObjectPointCloudsFunc(ctx, cameraName, params)
}
