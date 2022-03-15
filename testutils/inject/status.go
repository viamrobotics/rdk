package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/status"
)

// StatusService represents a fake instance of a Status service.
type StatusService struct {
	status.Service

	GetStatusFunc func(ctx context.Context, resources []resource.Name) ([]status.Status, error)
}

// GetStatus call the injected GetStatus or the real one.
func (s StatusService) GetStatus(ctx context.Context, resources []resource.Name) ([]status.Status, error) {
	if s.GetStatusFunc == nil {
		return s.Service.GetStatus(ctx, resources)
	}
	return s.GetStatusFunc(ctx, resources)
}
