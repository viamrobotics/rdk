//go:build !cgo

package web

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/utils/rpc"
)

// New returns a new web service for the given robot.
func New(r robot.Robot, logger golog.Logger, opts ...Option) Service {
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

	mu                      sync.Mutex
	r                       robot.Robot
	rpcServer               rpc.Server
	modServer               rpc.Server
	services                map[resource.API]resource.APIResourceCollection[resource.Resource]
	opts                    options
	addr                    string
	modAddr                 string
	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	isRunning               bool
	activeBackgroundWorkers sync.WaitGroup
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
func (svc *webService) initStreamServer(ctx context.Context, options weboptions.Options) error {
	return nil
}

// stub for missing gostream
type options struct{}
