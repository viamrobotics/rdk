//go:build no_cgo && !android

package web

import (
	"context"

	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/utils/rpc"
)

// New returns a new web service for the given robot.
func New(r robot.Robot, logger logging.Logger, opts ...Option) Service {
	var wOpts options
	for _, opt := range opts {
		opt.apply(&wOpts)
	}
	webSvc := &webService{
		Named:              InternalServiceName.AsNamed(),
		r:                  r,
		logger:             logger,
		rpcServer:          nil,
		services:           map[resource.API]resource.APIResourceCollection[resource.Resource]{},
		modPeerConnTracker: grpc.NewModPeerConnTracker(),
		opts:               wOpts,
	}
	return webSvc
}

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
func (svc *webService) initStreamServer(ctx context.Context) error {
	return nil
}

// stub implementation when gostream is not available.
func (svc *webService) initStreamServerForModule(_ context.Context, _ rpc.Server) error {
	return nil
}

// stub for missing gostream
type options struct{}
