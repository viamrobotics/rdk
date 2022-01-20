// Package registry operates the global registry of robotic parts.
package registry

import (
	"context"
	"fmt"
	"runtime"

	"github.com/edaniels/golog"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
)

type (
	// A CreateService creates a service from a given config.
	CreateService func(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error)
)

// RegDebugInfo represents some runtime information about the registration used
// for debugging purposes.
type RegDebugInfo struct {
	RegistrarLoc string
}

// Service stores a Service constructor (mandatory) and an attribute converter.
type Service struct {
	RegDebugInfo
	Constructor           CreateService
	AttributeMapConverter config.AttributeMapConverter
}

func getCallerName() string {
	pc, _, _, ok := runtime.Caller(2)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		return details.Name()
	}
	return "unknown"
}

// RegisterService registers a service type to a registration.
func RegisterService(subtype resource.Subtype, creator Service) {
	creator.RegistrarLoc = getCallerName()
	_, old := serviceRegistry[subtype.String()]
	if old {
		panic(errors.Errorf("trying to register two services with same subtype: %s", subtype))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype: %s", subtype))
	}
	serviceRegistry[subtype.String()] = creator
}

// ServiceLookup looks up a service registration by the given type. nil is returned if
// there is no registration.
func ServiceLookup(subtype resource.Subtype) *Service {
	registration, ok := serviceRegistry[subtype.String()]
	if ok {
		return &registration
	}
	return nil
}

type (
	// A CreateComponent creates a resource from a given config.
	CreateComponent func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error)

	// A CreateReconfigurable makes a reconfigurable resource from a given resource.
	CreateReconfigurable func(resource interface{}) (resource.Reconfigurable, error)

	// A RegisterSubtypeRPCService will register the subtype service to the grpc server.
	RegisterSubtypeRPCService func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error

	// A CreateRPCClient will create the client for the resource.
	// TODO: Remove as part of #227.
	CreateRPCClient func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{}
)

// Component stores a resource constructor (mandatory) and a Frame building function (optional).
type Component struct {
	RegDebugInfo
	Constructor CreateComponent
}

// ResourceSubtype stores subtype-specific functions and clients.
type ResourceSubtype struct {
	Reconfigurable     CreateReconfigurable
	RegisterRPCService RegisterSubtypeRPCService
	RPCClient          CreateRPCClient
}

// SubtypeGrpc stores functions necessary for a resource subtype to be accessible through grpc.
type SubtypeGrpc struct{}

// all registries.
var (
	componentRegistry = map[string]Component{}
	subtypeRegistry   = map[resource.Subtype]ResourceSubtype{}
	serviceRegistry   = map[string]Service{}
)

// RegisterComponent register a creator to its corresponding component and model.
func RegisterComponent(subtype resource.Subtype, model string, creator Component) {
	creator.RegistrarLoc = getCallerName()
	qName := fmt.Sprintf("%s/%s", subtype, model)
	_, old := componentRegistry[qName]
	if old {
		panic(errors.Errorf("trying to register two resources with same subtype:%s, model:%s", subtype, model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype:%s, model:%s", subtype, model))
	}
	componentRegistry[qName] = creator
}

// ComponentLookup looks up a creator by the given subtype and model. nil is returned if
// there is no creator registered.
func ComponentLookup(subtype resource.Subtype, model string) *Component {
	qName := fmt.Sprintf("%s/%s", subtype, model)
	if registration, ok := componentRegistry[qName]; ok {
		return &registration
	}
	return nil
}

// RegisterResourceSubtype register a ResourceSubtype to its corresponding component subtype.
func RegisterResourceSubtype(subtype resource.Subtype, creator ResourceSubtype) {
	_, old := subtypeRegistry[subtype]
	if old {
		panic(errors.Errorf("trying to register two of the same component subtype:%s", subtype))
	}
	if creator.Reconfigurable == nil && creator.RegisterRPCService == nil && creator.RPCClient == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype:%s", subtype))
	}
	subtypeRegistry[subtype] = creator
}

// ResourceSubtypeLookup looks up a ResourceSubtype by the given subtype. nil is returned if
// there is None.
func ResourceSubtypeLookup(subtype resource.Subtype) *ResourceSubtype {
	if registration, ok := subtypeRegistry[subtype]; ok {
		return &registration
	}
	return nil
}

// RegisteredServices returns a copy of the registered services.
func RegisteredServices() map[string]Service {
	copied, err := copystructure.Copy(serviceRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Service)
}

// RegisteredComponents returns a copy of the registered components.
func RegisteredComponents() map[string]Component {
	copied, err := copystructure.Copy(componentRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Component)
}

// RegisteredResourceSubtypes returns a copy of the registered resource subtypes.
func RegisteredResourceSubtypes() map[resource.Subtype]ResourceSubtype {
	copied, err := copystructure.Copy(subtypeRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[resource.Subtype]ResourceSubtype)
}
