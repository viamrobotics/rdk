package inject

import (
	"context"
	"image"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/objectdetection"
)

// VisionService represents a fake instance of a vision service.
type VisionService struct {
	vision.Service
	// detection functions
	GetDetectorNamesFunc        func(ctx context.Context) ([]string, error)
	AddDetectorFunc             func(ctx context.Context, cfg vision.DetectorConfig) error
	GetDetectionsFromCameraFunc func(ctx context.Context, cameraName, detectorName string) ([]objectdetection.Detection, error)
	GetDetectionsFunc           func(ctx context.Context, img image.Image, detectorName string) ([]objectdetection.Detection, error)
	// segmentation functions
	GetSegmenterNamesFunc      func(ctx context.Context) ([]string, error)
	GetSegmenterParametersFunc func(ctx context.Context, segmenterName string) ([]utils.TypedName, error)
	GetObjectPointCloudsFunc   func(ctx context.Context,
		cameraName, segmenterName string,
		params config.AttributeMap) ([]*viz.Object, error)
}

// GetDetectorNames calls the injected DetectorNames or the real variant.
func (vs *VisionService) GetDetectorNames(ctx context.Context) ([]string, error) {
	if vs.GetDetectorNamesFunc == nil {
		return vs.Service.GetDetectorNames(ctx)
	}
	return vs.GetDetectorNamesFunc(ctx)
}

// AddDetector calls the injected AddDetector or the real variant.
func (vs *VisionService) AddDetector(ctx context.Context, cfg vision.DetectorConfig) error {
	if vs.AddDetectorFunc == nil {
		return vs.Service.AddDetector(ctx, cfg)
	}
	return vs.AddDetectorFunc(ctx, cfg)
}

// GetDetectionsFromCamera calls the injected Detect or the real variant.
func (vs *VisionService) GetDetectionsFromCamera(ctx context.Context,
	cameraName, detectorName string,
) ([]objectdetection.Detection, error) {
	if vs.GetDetectionsFromCameraFunc == nil {
		return vs.Service.GetDetectionsFromCamera(ctx, cameraName, detectorName)
	}
	return vs.GetDetectionsFromCameraFunc(ctx, cameraName, detectorName)
}

// GetDetections calls the injected Detect or the real variant.
func (vs *VisionService) GetDetections(ctx context.Context, img image.Image, detectorName string,
) ([]objectdetection.Detection, error) {
	if vs.GetDetectionsFunc == nil {
		return vs.Service.GetDetections(ctx, img, detectorName)
	}
	return vs.GetDetectionsFunc(ctx, img, detectorName)
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

// GetSegmenterNames calls the injected GetSegmenterNames or the real variant.
func (vs *VisionService) GetSegmenterNames(ctx context.Context) ([]string, error) {
	if vs.GetSegmenterNamesFunc == nil {
		return vs.Service.GetSegmenterNames(ctx)
	}
	return vs.GetSegmenterNamesFunc(ctx)
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
