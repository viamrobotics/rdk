//go:build no_cgo && !android

package web

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/utils/rpc"
)

// Update updates the web service when the robot has changed. Without cgo (and
// therefore without video streams) it is a noop.
func (svc *webService) Reconfigure(ctx context.Context, _ resource.Dependencies, _ resource.Config) error {
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
