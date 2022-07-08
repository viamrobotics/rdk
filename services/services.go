package services

import (
	"context"
	"sync"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/utils"
	viamutils "go.viam.com/utils"
)

// type Service interface {
// 	Close(ctx context.Context) error
// }

type ReconfigurableService struct {
	mu     sync.RWMutex
	Actual interface{}
}

var (
	_ = resource.Reconfigurable(&ReconfigurableService{})
)

//function so that it can be passed into here
func (svc *ReconfigurableService) Reconfigure(ctx context.Context, newService resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newService.(*ReconfigurableService)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newService)
	}
	if err := viamutils.TryClose(ctx, svc.Actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.Actual = rSvc.Actual
	return nil
}

// WrapWithReconfigurable converts a service implementation to a reconfigurableService
// If service is already a Reconfigurable, then nothing is done.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	if reconfigurable, ok := s.(resource.Reconfigurable); ok {
		return reconfigurable, nil
	}

	return &ReconfigurableService{Actual: s}, nil
}
