package inject

import (
	"context"

	"go.viam.com/rdk/services/discovery"
)

// DiscoveryService represents a fake instance of a Discovery service.
type DiscoveryService struct {
	discovery.Service

	DiscoverFunc func(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error)
}

// Discover call the injected Discover or the real one.
func (s DiscoveryService) Discover(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error) {
	if s.DiscoverFunc == nil {
		return s.Service.Discover(ctx, keys)
	}
	return s.DiscoverFunc(ctx, keys)
}
