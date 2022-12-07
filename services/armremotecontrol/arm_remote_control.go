// Package armremotecontrol implements a remote control for a arm.
package armremotecontrol

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("arm_remote_control") // resource name

// Subtype is a constant that identifies the remote control resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named arm remote control service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
	})
}

// A Service controls the armremotecontrol for a robot.
type Service interface {
	Close(ctx context.Context) error
}

var _ = resource.Reconfigurable(&reconfigurableArmRemoteControl{})

type reconfigurableArmRemoteControl struct {
	mu     sync.RWMutex
	name   resource.Name
	actual Service
}

func (svc *reconfigurableArmRemoteControl) Name() resource.Name {
	return svc.name
}

func (svc *reconfigurableArmRemoteControl) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return utils.TryClose(ctx, svc.actual)
}

func (svc *reconfigurableArmRemoteControl) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableArmRemoteControl)
	if !ok {
		return rdkutils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := utils.TryClose(ctx, svc.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a ArmRemoteControl as a Reconfigurable.
func WrapWithReconfigurable(s interface{}, name resource.Name) (resource.Reconfigurable, error) {
	if reconfigurable, ok := s.(*reconfigurableArmRemoteControl); ok {
		return reconfigurable, nil
	}

	svc, ok := s.(Service)
	if !ok {
		return nil, rdkutils.NewUnimplementedInterfaceError("armremotecontrol.Service", s)
	}

	return &reconfigurableArmRemoteControl{name: name, actual: svc}, nil
}
