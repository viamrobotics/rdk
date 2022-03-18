package inject

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/objectsegmentation"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// ObjectSegmentationService represents a fake instance of an object segmentation
// service.
type ObjectSegmentationService struct {
	objectsegmentation.Service
	GetSegmentersFunc          func(ctx context.Context) ([]string, error)
	GetSegmenterParametersFunc func(ctx context.Context, segmenterName string) ([]utils.TypedName, error)
	GetObjectPointCloudsFunc   func(ctx context.Context,
		cameraName, segmenterName string,
		params config.AttributeMap) ([]*vision.Object, error)
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

// GetSegmenters calls the injected GetSegmenters or the real variant.
func (seg *ObjectSegmentationService) GetSegmenters(ctx context.Context) ([]string, error) {
	if seg.GetSegmentersFunc == nil {
		return seg.Service.GetSegmenters(ctx)
	}
	return seg.GetSegmentersFunc(ctx)
}

// GetSegmenterParameters calls the injected GetSegmenterParameters or the real variant.
func (seg *ObjectSegmentationService) GetSegmenterParameters(
	ctx context.Context,
	segmenterName string,
) ([]utils.TypedName, error) {
	if seg.GetSegmenterParametersFunc == nil {
		return seg.Service.GetSegmenterParameters(ctx, segmenterName)
	}
	return seg.GetSegmenterParametersFunc(ctx, segmenterName)
}
