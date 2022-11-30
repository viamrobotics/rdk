// Package baseremotecontrol implements a remote control for a base.
package baseremotecontrol

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("base_remote_control")

// Subtype is a constant that identifies the remote control resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named base remote control service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
	})
}

var (
	_ = resource.Reconfigurable(&reconfigurableBaseRemoteControl{})
	_ = viamutils.ContextCloser(&reconfigurableBaseRemoteControl{})
)

// A Service is the basis for the base remote control.
type Service interface {
	// Close out of all remote control related systems.
	Close(ctx context.Context) error
	// controllerInputs returns the list of inputs from the controller that are being monitored for that control mode.
	ControllerInputs() []input.Control
}

type reconfigurableBaseRemoteControl struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Service
}

func (svc *reconfigurableBaseRemoteControl) Name() resource.Name {
	return svc.name
}

func (svc *reconfigurableBaseRemoteControl) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return viamutils.TryClose(ctx, svc.actual)
}

func (svc *reconfigurableBaseRemoteControl) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableBaseRemoteControl)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := viamutils.TryClose(ctx, svc.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a BaseRemoteControl as a Reconfigurable.
func WrapWithReconfigurable(s interface{}, name resource.Name) (resource.Reconfigurable, error) {
	if reconfigurable, ok := s.(*reconfigurableBaseRemoteControl); ok {
		return reconfigurable, nil
	}
	svc, ok := s.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("baseremotecontrol.Service", s)
	}

	return &reconfigurableBaseRemoteControl{name: name, actual: svc}, nil
}
