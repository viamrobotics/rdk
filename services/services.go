package services

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/utils"
	viamutils "go.viam.com/utils"
)

type ReconfigurableService struct {
	mu     sync.RWMutex
	name   resource.Name
	Actual *interface{}
}

var (
	_ = resource.Reconfigurable(&ReconfigurableService{})
)

// function so that it can be passed into here
func (svc *ReconfigurableService) Reconfigure(ctx context.Context, newService resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	fmt.Println("in reconfigure")

	rSvc, ok := newService.(*ReconfigurableService)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newService)
	}

	// save this for future reference. This technically works if we know the
	// resources are of the same subtype/interface, so that they have the same
	// methods. but subtypes is not a SAFE way of checking since there could
	// be user error when someone new implements this
	if rSvc.name.Subtype.ResourceSubtype != svc.name.Subtype.ResourceSubtype {
		return errors.New("not the same resource subtype")
	}

	if err := viamutils.TryClose(ctx, svc.Actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}

	// check that the old and the new services are of the same type
	// if reflect.TypeOf(rSvc.Actual) != reflect.TypeOf(svc.Actual) {
	// 	return utils.NewUnexpectedTypeError(svc.Actual, rSvc.Actual)
	// }

	// see if this fails if they are actually different sizes
	fmt.Println(*(svc.Actual))
	fmt.Println(*(rSvc.Actual))
	fmt.Println(&(*(svc.Actual)))
	fmt.Println(&(*(rSvc.Actual)))
	*(svc.Actual) = *(rSvc.Actual)
	fmt.Println(*(svc.Actual))
	fmt.Println((svc.Actual))
	fmt.Println("done reconfigure")
	return nil
}

// WrapWithReconfigurable converts a service implementation to a reconfigurableService
// If service is already a Reconfigurable, then nothing is done.
func WrapWithReconfigurable(s interface{}, name resource.Name) (resource.Reconfigurable, error) {
	if reconfigurable, ok := s.(resource.Reconfigurable); ok {
		return reconfigurable, nil
	}

	return &ReconfigurableService{Actual: &s, name: name}, nil
}
