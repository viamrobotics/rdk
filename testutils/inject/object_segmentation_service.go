package inject

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/objectsegmentation"
	"go.viam.com/rdk/vision"
)

// ObjectSegmentationService represents a fake instance of an object segmentation
// service.
type ObjectSegmentationService struct {
	objectsegmentation.Service
	GetObjectPointCloudsFunc func(ctx context.Context,
		cameraName, segmenterName string,
		params config.AttributeMap) ([]*vision.Object, error)
	GetSegmenterParametersFunc func(ctx context.Context, segmenterName string) ([]string, error)
}

// GetObjectPointClouds calls the injected GetObjectPointClouds or the real variant.
func (seg *ObjectSegmentationService) GetObjectPointClouds(
	ctx context.Context,
	cameraName, segmenterName string,
	params config.AttributeMap,
) ([]*vision.Object, error) {
	if seg.GetObjectPointCloudsFunc == nil {
		return seg.Service.GetObjectPointClouds(ctx, cameraName, segmenterName, params)
	}
	return seg.GetObjectPointCloudsFunc(ctx, cameraName, segmenterName, params)
}

// GetSegmenterParameters calls the injected GetSegmenterParameters or the real variant.
func (seg *ObjectSegmentationService) GetSegmenterParameters(
	ctx context.Context,
	segmenterName string,
) ([]string, error) {
	if seg.GetSegmenterParametersFunc == nil {
		return seg.Service.GetSegmenterParameters(ctx, segmenterName)
	}
	return seg.GetSegmenterParametersFunc(ctx, segmenterName)
}
