package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
)

// DiscoveryService is an injectable discovery service.
type DiscoveryService struct {
	discovery.Service
	name                  resource.Name
	DiscoverResourcesFunc func(ctx context.Context, extra map[string]any) ([]resource.Config, error)
	DoFunc                func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// NewDiscoveryService returns a new injected discovery service.
func NewDiscoveryService(name string) *DiscoveryService {
	return &DiscoveryService{name: discovery.Named(name)}
}

// Name returns the name of the resource.
func (disSvc *DiscoveryService) Name() resource.Name {
	return disSvc.name
}

// DiscoverResources calls the injected DiscoverResourcesFunc or the real version.
func (disSvc *DiscoveryService) DiscoverResources(ctx context.Context, extra map[string]any) ([]resource.Config, error) {
	if disSvc.DiscoverResourcesFunc == nil {
		return disSvc.Service.DiscoverResources(ctx, extra)
	}
	return disSvc.DiscoverResourcesFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (disSvc *DiscoveryService) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if disSvc.DoFunc == nil {
		return disSvc.Service.DoCommand(ctx, cmd)
	}
	return disSvc.DoFunc(ctx, cmd)
}
