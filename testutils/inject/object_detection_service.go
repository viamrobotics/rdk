package inject

import (
	"context"

	"go.viam.com/rdk/services/objectdetection"
)

// ObjectDetectionService represents a fake instance of an object detection service.
type ObjectDetectionService struct {
	objectdetection.Service
	GetDetectorsFunc func(ctx context.Context) ([]string, error)
}

// GetDetectors calls the injected GetDetectors or the real variant.
func (seg *ObjectDetectionService) GetDetectors(ctx context.Context) ([]string, error) {
	if seg.GetDetectorsFunc == nil {
		return seg.Service.GetDetectors(ctx)
	}
	return seg.GetDetectorsFunc(ctx)
}
