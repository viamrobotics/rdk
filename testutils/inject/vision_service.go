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
	AddDetectorFunc   func(ctx context.Context, cfg vision.DetectorConfig) (bool, error)
	DetectFunc        func(ctx context.Context, cameraName, detectorName string) ([]objectdetection.Detection, error)
	// segmentation functions
	GetSegmentersFunc          func(ctx context.Context) ([]string, error)
	GetSegmenterParametersFunc func(ctx context.Context, segmenterName string) ([]utils.TypedName, error)
	GetObjectPointCloudsFunc   func(ctx context.Context,
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
func (vs *VisionService) AddDetector(ctx context.Context, cfg vision.DetectorConfig) (bool, error) {
	if vs.DetectorNamesFunc == nil {
		return vs.Service.AddDetector(ctx, cfg)
	}
	return vs.AddDetectorFunc(ctx, cfg)
}

// Detect calls the injected Detect or the real variant.
func (vs *VisionService) Detect(ctx context.Context, cameraName, detectorName string) ([]objectdetection.Detection, error) {
	if vs.DetectFunc == nil {
		return vs.Service.Detect(ctx, cameraName, detectorName)
	}
	return vs.DetectFunc(ctx, cameraName, detectorName)
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

// GetSegmenters calls the injected GetSegmenters or the real variant.
func (vs *VisionService) GetSegmenters(ctx context.Context) ([]string, error) {
	if vs.GetSegmentersFunc == nil {
		return vs.Service.GetSegmenters(ctx)
	}
	return vs.GetSegmentersFunc(ctx)
}

// GetSegmenterParameters calls the injected GetSegmenterParameters or the real variant.
func (vs *VisionService) GetSegmenterParameters(
	ctx context.Context,
	segmenterName string,
) ([]utils.TypedName, error) {
	if vs.GetSegmenterParametersFunc == nil {
		return vs.Service.GetSegmenterParameters(ctx, segmenterName)
	}
	return vs.GetSegmenterParametersFunc(ctx, segmenterName)
}
