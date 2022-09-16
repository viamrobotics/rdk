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
	GetModelParameterSchemaFunc func(ctx context.Context, modelType vision.VisModelType) (*jsonschema.Schema, error)
	// detection functions
	GetDetectorNamesFunc     func(ctx context.Context) ([]string, error)
	AddDetectorFunc          func(ctx context.Context, cfg vision.VisModelConfig) error
	RemoveDetectorFunc       func(ctx context.Context, detectorName string) error
	DetectionsFromCameraFunc func(ctx context.Context, cameraName, detectorName string) ([]objectdetection.Detection, error)
	DetectionsFunc           func(ctx context.Context, img image.Image, detectorName string) ([]objectdetection.Detection, error)
	// classification functions
	ClassifierNamesFunc           func(ctx context.Context) ([]string, error)
	AddClassifierFunc             func(ctx context.Context, cfg vision.VisModelConfig) error
	RemoveClassifierFunc          func(ctx context.Context, classifierName string) error
	ClassificationsFromCameraFunc func(ctx context.Context, cameraName, classifierName string,
		n int) (classification.Classifications, error)
	ClassificationsFunc func(ctx context.Context, img image.Image, classifierName string,
		n int) (classification.Classifications, error)

	// segmentation functions
	SegmenterNamesFunc       func(ctx context.Context) ([]string, error)
	AddSegmenterFunc         func(ctx context.Context, cfg vision.VisModelConfig) error
	RemoveSegmenterFunc      func(ctx context.Context, segmenterName string) error
	GetObjectPointCloudsFunc func(ctx context.Context, cameraName, segmenterName string) ([]*viz.Object, error)
}

// GetModelParameterSchema calls the injected ModelParameters or the real variant.
func (vs *VisionService) GetModelParameterSchema(ctx context.Context, modelType vision.VisModelType) (*jsonschema.Schema, error) {
	if vs.GetDetectorNamesFunc == nil {
		return vs.Service.GetModelParameterSchema(ctx, modelType)
	}
	return vs.GetModelParameterSchema(ctx, modelType)
}

// DetectorNames calls the injected DetectorNames or the real variant.
func (vs *VisionService) DetectorNames(ctx context.Context) ([]string, error) {
	if vs.GetDetectorNamesFunc == nil {
		return vs.Service.DetectorNames(ctx)
	}
	return vs.GetDetectorNamesFunc(ctx)
}

// AddDetector calls the injected AddDetector or the real variant.
func (vs *VisionService) AddDetector(ctx context.Context, cfg vision.VisModelConfig) error {
	if vs.AddDetectorFunc == nil {
		return vs.Service.AddDetector(ctx, cfg)
	}
	return vs.AddDetectorFunc(ctx, cfg)
}

// RemoveDetector calls the injected RemoveDetector or the real variant.
func (vs *VisionService) RemoveDetector(ctx context.Context, detectorName string) error {
	if vs.RemoveDetectorFunc == nil {
		return vs.Service.RemoveDetector(ctx, detectorName)
	}
	return vs.RemoveDetectorFunc(ctx, detectorName)
}

// DetectionsFromCamera calls the injected Detector or the real variant.
func (vs *VisionService) DetectionsFromCamera(ctx context.Context,
	cameraName, detectorName string,
) ([]objectdetection.Detection, error) {
	if vs.DetectionsFromCameraFunc == nil {
		return vs.Service.DetectionsFromCamera(ctx, cameraName, detectorName)
	}
	return vs.DetectionsFromCameraFunc(ctx, cameraName, detectorName)
}

// Detections calls the injected Detect or the real variant.
func (vs *VisionService) Detections(ctx context.Context, img image.Image, detectorName string,
) ([]objectdetection.Detection, error) {
	if vs.DetectionsFunc == nil {
		return vs.Service.Detections(ctx, img, detectorName)
	}
	return vs.DetectionsFunc(ctx, img, detectorName)
}

// ClassifierNames calls the injected ClassifierNames or the real variant.
func (vs *VisionService) ClassifierNames(ctx context.Context) ([]string, error) {
	if vs.ClassifierNamesFunc == nil {
		return vs.Service.ClassifierNames(ctx)
	}
	return vs.ClassifierNamesFunc(ctx)
}

// AddClassifier calls the injected AddClassifier or the real variant.
func (vs *VisionService) AddClassifier(ctx context.Context, cfg vision.VisModelConfig) error {
	if vs.AddClassifierFunc == nil {
		return vs.Service.AddClassifier(ctx, cfg)
	}
	return vs.AddClassifierFunc(ctx, cfg)
}

// RemoveClassifier calls the injected RemoveClassifier or the real variant.
func (vs *VisionService) RemoveClassifier(ctx context.Context, classifierName string) error {
	if vs.RemoveClassifierFunc == nil {
		return vs.Service.RemoveClassifier(ctx, classifierName)
	}
	return vs.RemoveClassifierFunc(ctx, classifierName)
}

// ClassificationsFromCamera calls the injected Classifer or the real variant.
func (vs *VisionService) ClassificationsFromCamera(ctx context.Context,
	cameraName, classifierName string, n int,
) (classification.Classifications, error) {
	if vs.ClassificationsFromCameraFunc == nil {
		return vs.Service.ClassificationsFromCamera(ctx, cameraName, classifierName, n)
	}
	return vs.ClassificationsFromCameraFunc(ctx, cameraName, classifierName, n)
}

// Classifications calls the injected Classifier or the real variant.
func (vs *VisionService) Classifications(ctx context.Context, img image.Image,
	classifierName string, n int,
) (classification.Classifications, error) {
	if vs.ClassificationsFunc == nil {
		return vs.Service.Classifications(ctx, img, classifierName, n)
	}
	return vs.ClassificationsFunc(ctx, img, classifierName, n)
}

// GetObjectPointClouds calls the injected GetObjectPointClouds or the real variant.
func (vs *VisionService) GetObjectPointClouds(
	ctx context.Context,
	cameraName, segmenterName string,
) ([]*viz.Object, error) {
	if vs.GetObjectPointCloudsFunc == nil {
		return vs.Service.GetObjectPointClouds(ctx, cameraName, segmenterName)
	}
	return vs.GetObjectPointCloudsFunc(ctx, cameraName, segmenterName)
}

// SegmenterNames calls the injected SegmenterNames or the real variant.
func (vs *VisionService) SegmenterNames(ctx context.Context) ([]string, error) {
	if vs.SegmenterNamesFunc == nil {
		return vs.Service.SegmenterNames(ctx)
	}
	return vs.SegmenterNamesFunc(ctx)
}

// AddSegmenter calls the injected AddSegmenter or the real variant.
func (vs *VisionService) AddSegmenter(ctx context.Context, cfg vision.VisModelConfig) error {
	if vs.AddSegmenterFunc == nil {
		return vs.Service.AddSegmenter(ctx, cfg)
	}
	return vs.AddSegmenterFunc(ctx, cfg)
}

// RemoveSegmenter calls the injected RemoveSegmenter or the real variant.
func (vs *VisionService) RemoveSegmenter(ctx context.Context, segmenterName string) error {
	if vs.RemoveSegmenterFunc == nil {
		return vs.Service.RemoveSegmenter(ctx, segmenterName)
	}
	return vs.RemoveSegmenterFunc(ctx, segmenterName)
}
