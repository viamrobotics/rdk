package inject

import (
	"context"
	"image"

	"github.com/invopop/jsonschema"

	"go.viam.com/rdk/services/vision"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
)

// VisionService represents a fake instance of a vision service.
type VisionService struct {
	vision.Service
	GetModelParameterSchemaFunc func(
		ctx context.Context, modelType vision.VisModelType, extra map[string]interface{},
	) (*jsonschema.Schema, error)
	// detection functions
	GetDetectorNamesFunc     func(ctx context.Context, extra map[string]interface{}) ([]string, error)
	AddDetectorFunc          func(ctx context.Context, cfg vision.VisModelConfig, extra map[string]interface{}) error
	RemoveDetectorFunc       func(ctx context.Context, detectorName string, extra map[string]interface{}) error
	DetectionsFromCameraFunc func(
		ctx context.Context, cameraName, detectorName string, extra map[string]interface{},
	) ([]objectdetection.Detection, error)
	DetectionsFunc func(
		ctx context.Context, img image.Image, detectorName string, extra map[string]interface{},
	) ([]objectdetection.Detection, error)
	// classification functions
	ClassifierNamesFunc           func(ctx context.Context, extra map[string]interface{}) ([]string, error)
	AddClassifierFunc             func(ctx context.Context, cfg vision.VisModelConfig, extra map[string]interface{}) error
	RemoveClassifierFunc          func(ctx context.Context, classifierName string, extra map[string]interface{}) error
	ClassificationsFromCameraFunc func(ctx context.Context, cameraName, classifierName string,
		n int, extra map[string]interface{}) (classification.Classifications, error)
	ClassificationsFunc func(ctx context.Context, img image.Image, classifierName string,
		n int, extra map[string]interface{}) (classification.Classifications, error)

	// segmentation functions
	SegmenterNamesFunc       func(ctx context.Context, extra map[string]interface{}) ([]string, error)
	AddSegmenterFunc         func(ctx context.Context, cfg vision.VisModelConfig, extra map[string]interface{}) error
	RemoveSegmenterFunc      func(ctx context.Context, segmenterName string, extra map[string]interface{}) error
	GetObjectPointCloudsFunc func(ctx context.Context, cameraName, segmenterName string, extra map[string]interface{}) ([]*viz.Object, error)
	DoCommandFunc            func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
}

// GetModelParameterSchema calls the injected ModelParameters or the real variant.
func (vs *VisionService) GetModelParameterSchema(
	ctx context.Context,
	modelType vision.VisModelType,
	extra map[string]interface{},
) (*jsonschema.Schema, error) {
	if vs.GetDetectorNamesFunc == nil {
		return vs.Service.GetModelParameterSchema(ctx, modelType, extra)
	}
	return vs.GetModelParameterSchema(ctx, modelType, extra)
}

// DetectorNames calls the injected DetectorNames or the real variant.
func (vs *VisionService) DetectorNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	if vs.GetDetectorNamesFunc == nil {
		return vs.Service.DetectorNames(ctx, extra)
	}
	return vs.GetDetectorNamesFunc(ctx, extra)
}

// AddDetector calls the injected AddDetector or the real variant.
func (vs *VisionService) AddDetector(ctx context.Context, cfg vision.VisModelConfig, extra map[string]interface{}) error {
	if vs.AddDetectorFunc == nil {
		return vs.Service.AddDetector(ctx, cfg, extra)
	}
	return vs.AddDetectorFunc(ctx, cfg, extra)
}

// RemoveDetector calls the injected RemoveDetector or the real variant.
func (vs *VisionService) RemoveDetector(ctx context.Context, detectorName string, extra map[string]interface{}) error {
	if vs.RemoveDetectorFunc == nil {
		return vs.Service.RemoveDetector(ctx, detectorName, extra)
	}
	return vs.RemoveDetectorFunc(ctx, detectorName, extra)
}

// DetectionsFromCamera calls the injected Detector or the real variant.
func (vs *VisionService) DetectionsFromCamera(ctx context.Context,
	cameraName, detectorName string, extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	if vs.DetectionsFromCameraFunc == nil {
		return vs.Service.DetectionsFromCamera(ctx, cameraName, detectorName, extra)
	}
	return vs.DetectionsFromCameraFunc(ctx, cameraName, detectorName, extra)
}

// Detections calls the injected Detect or the real variant.
func (vs *VisionService) Detections(ctx context.Context, img image.Image, detectorName string, extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	if vs.DetectionsFunc == nil {
		return vs.Service.Detections(ctx, img, detectorName, extra)
	}
	return vs.DetectionsFunc(ctx, img, detectorName, extra)
}

// ClassifierNames calls the injected ClassifierNames or the real variant.
func (vs *VisionService) ClassifierNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	if vs.ClassifierNamesFunc == nil {
		return vs.Service.ClassifierNames(ctx, extra)
	}
	return vs.ClassifierNamesFunc(ctx, extra)
}

// AddClassifier calls the injected AddClassifier or the real variant.
func (vs *VisionService) AddClassifier(ctx context.Context, cfg vision.VisModelConfig, extra map[string]interface{}) error {
	if vs.AddClassifierFunc == nil {
		return vs.Service.AddClassifier(ctx, cfg, extra)
	}
	return vs.AddClassifierFunc(ctx, cfg, extra)
}

// RemoveClassifier calls the injected RemoveClassifier or the real variant.
func (vs *VisionService) RemoveClassifier(ctx context.Context, classifierName string, extra map[string]interface{}) error {
	if vs.RemoveClassifierFunc == nil {
		return vs.Service.RemoveClassifier(ctx, classifierName, extra)
	}
	return vs.RemoveClassifierFunc(ctx, classifierName, extra)
}

// ClassificationsFromCamera calls the injected Classifer or the real variant.
func (vs *VisionService) ClassificationsFromCamera(ctx context.Context,
	cameraName, classifierName string, n int, extra map[string]interface{},
) (classification.Classifications, error) {
	if vs.ClassificationsFromCameraFunc == nil {
		return vs.Service.ClassificationsFromCamera(ctx, cameraName, classifierName, n, extra)
	}
	return vs.ClassificationsFromCameraFunc(ctx, cameraName, classifierName, n, extra)
}

// Classifications calls the injected Classifier or the real variant.
func (vs *VisionService) Classifications(ctx context.Context, img image.Image,
	classifierName string, n int, extra map[string]interface{},
) (classification.Classifications, error) {
	if vs.ClassificationsFunc == nil {
		return vs.Service.Classifications(ctx, img, classifierName, n, extra)
	}
	return vs.ClassificationsFunc(ctx, img, classifierName, n, extra)
}

// GetObjectPointClouds calls the injected GetObjectPointClouds or the real variant.
func (vs *VisionService) GetObjectPointClouds(
	ctx context.Context,
	cameraName, segmenterName string, extra map[string]interface{},
) ([]*viz.Object, error) {
	if vs.GetObjectPointCloudsFunc == nil {
		return vs.Service.GetObjectPointClouds(ctx, cameraName, segmenterName, extra)
	}
	return vs.GetObjectPointCloudsFunc(ctx, cameraName, segmenterName, extra)
}

// SegmenterNames calls the injected SegmenterNames or the real variant.
func (vs *VisionService) SegmenterNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	if vs.SegmenterNamesFunc == nil {
		return vs.Service.SegmenterNames(ctx, extra)
	}
	return vs.SegmenterNamesFunc(ctx, extra)
}

// AddSegmenter calls the injected AddSegmenter or the real variant.
func (vs *VisionService) AddSegmenter(ctx context.Context, cfg vision.VisModelConfig, extra map[string]interface{}) error {
	if vs.AddSegmenterFunc == nil {
		return vs.Service.AddSegmenter(ctx, cfg, extra)
	}
	return vs.AddSegmenterFunc(ctx, cfg, extra)
}

// RemoveSegmenter calls the injected RemoveSegmenter or the real variant.
func (vs *VisionService) RemoveSegmenter(ctx context.Context, segmenterName string, extra map[string]interface{}) error {
	if vs.RemoveSegmenterFunc == nil {
		return vs.Service.RemoveSegmenter(ctx, segmenterName, extra)
	}
	return vs.RemoveSegmenterFunc(ctx, segmenterName, extra)
}

// DoCommand calls the injected DoCommand or the real variant.
func (vs *VisionService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if vs.DoCommandFunc == nil {
		return vs.Service.DoCommand(ctx, cmd)
	}
	return vs.DoCommandFunc(ctx, cmd)
}
