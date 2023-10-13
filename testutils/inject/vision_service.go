//go:build !no_media

package inject

import (
	"context"
	"image"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
)

// VisionService represents a fake instance of a vision service.
type VisionService struct {
	vision.Service
	name                     resource.Name
	DetectionsFromCameraFunc func(
		ctx context.Context, cameraName string, extra map[string]interface{},
	) ([]objectdetection.Detection, error)
	DetectionsFunc func(
		ctx context.Context, img image.Image, extra map[string]interface{},
	) ([]objectdetection.Detection, error)
	// classification functions
	ClassificationsFromCameraFunc func(ctx context.Context, cameraName string,
		n int, extra map[string]interface{}) (classification.Classifications, error)
	ClassificationsFunc func(ctx context.Context, img image.Image,
		n int, extra map[string]interface{}) (classification.Classifications, error)
	// segmentation functions
	GetObjectPointCloudsFunc func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error)
	DoCommandFunc            func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc func(ctx context.Context) error
}

// NewVisionService returns a new injected vision service.
func NewVisionService(name string) *VisionService {
	return &VisionService{name: vision.Named(name)}
}

// Name returns the name of the resource.
func (vs *VisionService) Name() resource.Name {
	return vs.name
}

// DetectionsFromCamera calls the injected DetectionsFromCamera or the real variant.
func (vs *VisionService) DetectionsFromCamera(ctx context.Context, cameraName string, extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	if vs.DetectionsFunc == nil {
		return vs.Service.DetectionsFromCamera(ctx, cameraName, extra)
	}
	return vs.DetectionsFromCameraFunc(ctx, cameraName, extra)
}

// Detections calls the injected Detect or the real variant.
func (vs *VisionService) Detections(ctx context.Context, img image.Image, extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	if vs.DetectionsFunc == nil {
		return vs.Service.Detections(ctx, img, extra)
	}
	return vs.DetectionsFunc(ctx, img, extra)
}

// ClassificationsFromCamera calls the injected Classifer or the real variant.
func (vs *VisionService) ClassificationsFromCamera(ctx context.Context,
	cameraName string, n int, extra map[string]interface{},
) (classification.Classifications, error) {
	if vs.ClassificationsFromCameraFunc == nil {
		return vs.Service.ClassificationsFromCamera(ctx, cameraName, n, extra)
	}
	return vs.ClassificationsFromCameraFunc(ctx, cameraName, n, extra)
}

// Classifications calls the injected Classifier or the real variant.
func (vs *VisionService) Classifications(ctx context.Context, img image.Image,
	n int, extra map[string]interface{},
) (classification.Classifications, error) {
	if vs.ClassificationsFunc == nil {
		return vs.Service.Classifications(ctx, img, n, extra)
	}
	return vs.ClassificationsFunc(ctx, img, n, extra)
}

// GetObjectPointClouds calls the injected GetObjectPointClouds or the real variant.
func (vs *VisionService) GetObjectPointClouds(
	ctx context.Context,
	cameraName string, extra map[string]interface{},
) ([]*viz.Object, error) {
	if vs.GetObjectPointCloudsFunc == nil {
		return vs.Service.GetObjectPointClouds(ctx, cameraName, extra)
	}
	return vs.GetObjectPointCloudsFunc(ctx, cameraName, extra)
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

// Close calls the injected Close or the real version.
func (vs *VisionService) Close(ctx context.Context) error {
	if vs.CloseFunc == nil {
		if vs.Service == nil {
			return nil
		}
		return vs.Service.Close(ctx)
	}
	return vs.CloseFunc(ctx)
}
