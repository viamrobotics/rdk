package services

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
	viamutils "go.viam.com/utils"
)

type Service interface {
	Close(ctx context.Context) error

	New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error)
}

type reconfigurableService struct {
	mu     sync.RWMutex
	actual Service
}

var (
	_ = resource.Reconfigurable(&reconfigurableService{})
)

//function so that it can be passed into here
func (svc *reconfigurableService) Reconfigure(ctx context.Context, newService resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newService.(*reconfigurableService)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newService)
	}
	if err := viamutils.TryClose(ctx, svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable converts a service implementation to a reconfigurableService
// If service is already a Reconfigurable, then nothing is done.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Service", s)
	}

	if reconfigurable, ok := svc.(resource.Reconfigurable); ok {
		return reconfigurable, nil
	}

	return &reconfigurableService{actual: svc}, nil
}
