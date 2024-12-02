//go:build no_cgo && !android

package web

import (
	"context"
	"sync"

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
		Named:     InternalServiceName.AsNamed(),
		r:         r,
		logger:    logger,
		rpcServer: nil,
		services:  map[resource.API]resource.APIResourceCollection[resource.Resource]{},
		opts:      wOpts,
	}
	return webSvc
}

type webService struct {
	resource.Named

	mu         sync.Mutex
	r          robot.Robot
	rpcServer  rpc.Server
	modServer  rpc.Server
	services   map[resource.API]resource.APIResourceCollection[resource.Resource]
	opts       options
	addr       string
	modAddr    string
	logger     logging.Logger
	cancelCtx  context.Context
	cancelFunc func()
	isRunning  bool
	webWorkers sync.WaitGroup
	modWorkers sync.WaitGroup
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

type stats struct {
	RPCServer any
}

func (svc *webService) Stats() any {
	if haveLock := svc.mu.TryLock(); !haveLock {
		return stats{nil}
	}
	defer svc.mu.Unlock()

	if svc.rpcServer == nil {
		return stats{nil}
	}

	return stats{svc.rpcServer.Stats()}
}

// stub for missing gostream
type options struct{}
