package inject

import (
	"context"

	"go.viam.com/rdk/services/objectdetection"
)

// ObjectDetectionService represents a fake instance of an object detection service.
type ObjectDetectionService struct {
	objectdetection.Service
	DetectorNamesFunc func(ctx context.Context) ([]string, error)
}

// DetectorNames calls the injected DetectorNames or the real variant.
func (seg *ObjectDetectionService) DetectorNames(ctx context.Context) ([]string, error) {
	if seg.DetectorNamesFunc == nil {
		return seg.Service.DetectorNames(ctx)
	}
	return seg.DetectorNamesFunc(ctx)
}
