//go:build no_cgo && !android

package web

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/utils/rpc"
)

// Update updates the web service when the robot has changed.
func (svc *webService) Reconfigure(ctx context.Context, deps resource.Dependencies, _ resource.Config) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if err := svc.updateResources(deps); err != nil {
		return err
	}
	return nil
}

// stub implementation when gostream not available
func (svc *webService) closeStreamServer() {}

// stub implementation when gostream not available
func (svc *webService) initStreamServer(_ context.Context, _ rpc.Server) error {
	return nil
}

// stub for missing gostream
type options struct{}
