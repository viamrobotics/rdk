package inject

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/objectdetection"
)

// VisionService represents a fake instance of an object detection service.
type VisionService struct {
	vision.Service
	DetectorNamesFunc func(ctx context.Context) ([]string, error)
	AddDetectorFunc   func(ctx context.Context, cfg vision.DetectorConfig) error
	GetDetectionsFunc func(ctx context.Context, cameraName, detectorName string) ([]objectdetection.Detection, error)
	// segmentation functions
	SegmenterNamesFunc       func(ctx context.Context) ([]string, error)
	SegmenterParametersFunc  func(ctx context.Context, segmenterName string) ([]utils.TypedName, error)
	GetObjectPointCloudsFunc func(ctx context.Context,
		cameraName, segmenterName string,
		params config.AttributeMap) ([]*viz.Object, error)
}

// DetectorNames calls the injected DetectorNames or the real variant.
func (vs *VisionService) DetectorNames(ctx context.Context) ([]string, error) {
	if vs.DetectorNamesFunc == nil {
		return vs.Service.DetectorNames(ctx)
	}
	return vs.DetectorNamesFunc(ctx)
}

// AddDetector calls the injected AddDetector or the real variant.
func (vs *VisionService) AddDetector(ctx context.Context, cfg vision.DetectorConfig) error {
	if vs.DetectorNamesFunc == nil {
		return vs.Service.AddDetector(ctx, cfg)
	}
	return vs.AddDetectorFunc(ctx, cfg)
}

// GetDetections calls the injected Detect or the real variant.
func (vs *VisionService) GetDetections(ctx context.Context, cameraName, detectorName string) ([]objectdetection.Detection, error) {
	if vs.GetDetectionsFunc == nil {
		return vs.Service.GetDetections(ctx, cameraName, detectorName)
	}
	return vs.GetDetectionsFunc(ctx, cameraName, detectorName)
}

// GetObjectPointClouds calls the injected GetObjectPointClouds or the real variant.
func (vs *VisionService) GetObjectPointClouds(
	ctx context.Context,
	cameraName, segmenterName string,
	params config.AttributeMap,
) ([]*viz.Object, error) {
	if vs.GetObjectPointCloudsFunc == nil {
		return vs.Service.GetObjectPointClouds(ctx, cameraName, segmenterName, params)
	}
	return vs.GetObjectPointCloudsFunc(ctx, cameraName, segmenterName, params)
}

// SegmenterNames calls the injected SegmenterNames or the real variant.
func (vs *VisionService) SegmenterNames(ctx context.Context) ([]string, error) {
	if vs.SegmenterNamesFunc == nil {
		return vs.Service.SegmenterNames(ctx)
	}
	return vs.SegmenterNamesFunc(ctx)
}

// SegmenterParameters calls the injected SegmenterParameters or the real variant.
func (vs *VisionService) SegmenterParameters(
	ctx context.Context,
	segmenterName string,
) ([]utils.TypedName, error) {
	if vs.SegmenterParametersFunc == nil {
		return vs.Service.SegmenterParameters(ctx, segmenterName)
	}
	return vs.SegmenterParametersFunc(ctx, segmenterName)
}
