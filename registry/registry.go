// Package registry operates the global registry of robotic parts.
package registry

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
)

type (
	// A CreateServiceWithRobot creates a resource from a robot and a given config.
	CreateServiceWithRobot func(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error)

	// A CreateService creates a resource from a collection of dependencies and a given config.
	CreateService func(ctx context.Context, deps Dependencies, config config.Service, logger golog.Logger) (interface{}, error)
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
	// This is a legacy constructor for default services
	RobotConstructor CreateServiceWithRobot
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
func RegisterService(subtype resource.Subtype, model resource.Model, creator Service) {
	registryMu.Lock()
	defer registryMu.Unlock()
	creator.RegistrarLoc = getCallerName()
	qName := fmt.Sprintf("%s/%s", subtype, model)
	_, old := serviceRegistry[qName]
	if old {
		panic(errors.Errorf("trying to register two services with same subtype:%s, model:%s", subtype, model))
	}
	if creator.Constructor == nil && creator.RobotConstructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype: %s", subtype))
	}
	serviceRegistry[qName] = creator
}

// DeregisterService removes a previously registered service.
func DeregisterService(subtype resource.Subtype, model resource.Model) {
	registryMu.Lock()
	defer registryMu.Unlock()
	qName := fmt.Sprintf("%s/%s", subtype, model)
	delete(serviceRegistry, qName)
}

// ServiceLookup looks up a service registration by the given type. nil is returned if
// there is no registration.
func ServiceLookup(subtype resource.Subtype, model resource.Model) *Service {
	registryMu.RLock()
	defer registryMu.RUnlock()
	qName := fmt.Sprintf("%s/%s", subtype, model)
	if registration, ok := RegisteredServices()[qName]; ok {
		return &registration
	}
	return nil
}

type (
	// Dependencies is a map of resources that a component requires for creation.
	Dependencies map[resource.Name]interface{}

	// A CreateComponentWithRobot creates a resource from a robot and a given config.
	CreateComponentWithRobot func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error)

	// A CreateComponent creates a resource from a collection of dependencies and a given config.
	CreateComponent func(ctx context.Context, deps Dependencies, config config.Component, logger golog.Logger) (interface{}, error)

	// A CreateReconfigurable makes a reconfigurable resource from a given resource.
	CreateReconfigurable func(resource interface{}, name resource.Name) (resource.Reconfigurable, error)

	// CreateStatus creates a status from a given resource. The return type is expected to be comprised of string keys
	// (or it should be possible to decompose it into string keys) and values comprised of primitives, list of primitives,
	// maps with string keys (or at least can be decomposed into one), or lists of the aforementioned type of maps.
	// Results with other types of data are not guaranteed.
	CreateStatus func(ctx context.Context, resource interface{}) (interface{}, error)

	// A RegisterSubtypeRPCService will register the subtype service to the grpc server.
	RegisterSubtypeRPCService func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error

	// A CreateRPCClient will create the client for the resource.
	CreateRPCClient func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{}
)

// A DependencyNotReadyError is used whenever we reference a dependency that has not been
// constructed and registered yet.
type DependencyNotReadyError struct {
	Name string
}

func (e *DependencyNotReadyError) Error() string {
	return fmt.Sprintf("dependency %q has not been registered yet", e.Name)
}

// Component stores a resource constructor (mandatory) and a Frame building function (optional).
type Component struct {
	RegDebugInfo
	Constructor CreateComponent
	// TODO(RSDK-418): remove this legacy constructor once all components that use it no longer need to receive the entire robot.
	RobotConstructor CreateComponentWithRobot
}

// ResourceSubtype stores subtype-specific functions and clients.
type ResourceSubtype struct {
	Reconfigurable        CreateReconfigurable
	Status                CreateStatus
	RegisterRPCService    RegisterSubtypeRPCService
	RPCServiceDesc        *grpc.ServiceDesc
	ReflectRPCServiceDesc *desc.ServiceDescriptor `copy:"shallow"`
	RPCClient             CreateRPCClient

	// MaxInstance sets a limit on the number of this subtype allowed on a robot.
	// If MaxInstance is not set then it will default to 0 and there will be no limit.
	MaxInstance int
}

// SubtypeGrpc stores functions necessary for a resource subtype to be accessible through grpc.
type SubtypeGrpc struct{}

// all registries.
var (
	registryMu        sync.RWMutex
	componentRegistry = map[string]Component{}
	subtypeRegistry   = map[resource.Subtype]ResourceSubtype{}
	serviceRegistry   = map[string]Service{}
)

// RegisterComponent register a creator to its corresponding component and model.
func RegisterComponent(subtype resource.Subtype, model resource.Model, creator Component) {
	registryMu.Lock()
	defer registryMu.Unlock()
	creator.RegistrarLoc = getCallerName()
	qName := fmt.Sprintf("%s/%s", subtype.String(), model.String())

	_, old := componentRegistry[qName]
	if old {
		panic(errors.Errorf("trying to register two resources with same subtype:%s, model:%s", subtype, model))
	}
	if creator.Constructor == nil && creator.RobotConstructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype:%s, model:%s", subtype, model))
	}
	componentRegistry[qName] = creator
}

// DeregisterComponent removes a previously registered component.
func DeregisterComponent(subtype resource.Subtype, model resource.Model) {
	registryMu.Lock()
	defer registryMu.Unlock()
	qName := fmt.Sprintf("%s/%s", subtype, model)
	delete(componentRegistry, qName)
}

// ComponentLookup looks up a creator by the given subtype and model. nil is returned if
// there is no creator registered.
func ComponentLookup(subtype resource.Subtype, model resource.Model) *Component {
	qName := fmt.Sprintf("%s/%s", subtype, model)

	if registration, ok := RegisteredComponents()[qName]; ok {
		return &registration
	}
	return nil
}

// RegisterResourceSubtype register a ResourceSubtype to its corresponding component subtype.
func RegisterResourceSubtype(subtype resource.Subtype, creator ResourceSubtype) {
	registryMu.Lock()
	defer registryMu.Unlock()
	_, old := subtypeRegistry[subtype]
	if old {
		panic(errors.Errorf("trying to register two of the same resource subtype: %s", subtype))
	}
	if creator.Reconfigurable == nil && creator.Status == nil &&
		creator.RegisterRPCService == nil && creator.RPCClient == nil &&
		creator.ReflectRPCServiceDesc == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype: %s", subtype))
	}
	if creator.RegisterRPCService != nil && creator.RPCServiceDesc == nil {
		panic(errors.Errorf("cannot register a RPC enabled subtype with no RPC service description: %s", subtype))
	}

	if creator.RPCServiceDesc != nil && creator.ReflectRPCServiceDesc == nil {
		reflectSvcDesc, err := grpcreflect.LoadServiceDescriptor(creator.RPCServiceDesc)
		if err != nil {
			panic(err)
		}
		creator.ReflectRPCServiceDesc = reflectSvcDesc
	}
	subtypeRegistry[subtype] = creator
}

// DeregisterResourceSubtype removes a previously registered subtype.
func DeregisterResourceSubtype(subtype resource.Subtype) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(subtypeRegistry, subtype)
}

// ResourceSubtypeLookup looks up a ResourceSubtype by the given subtype. nil is returned if
// there is None.
func ResourceSubtypeLookup(subtype resource.Subtype) *ResourceSubtype {
	if registration, ok := RegisteredResourceSubtypes()[subtype]; ok {
		return &registration
	}
	return nil
}

// RegisteredServices returns a copy of the registered services.
func RegisteredServices() map[string]Service {
	registryMu.RLock()
	defer registryMu.RUnlock()
	copied, err := copystructure.Copy(serviceRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Service)
}

// RegisteredComponents returns a copy of the registered components.
func RegisteredComponents() map[string]Component {
	registryMu.RLock()
	defer registryMu.RUnlock()
	copied, err := copystructure.Copy(componentRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Component)
}

// RegisteredResourceSubtypes returns a copy of the registered resource subtypes.
func RegisteredResourceSubtypes() map[resource.Subtype]ResourceSubtype {
	registryMu.RLock()
	defer registryMu.RUnlock()
	toCopy := make(map[resource.Subtype]ResourceSubtype, len(subtypeRegistry))
	for k, v := range subtypeRegistry {
		toCopy[k] = v
	}
	return toCopy
}

var discoveryFunctions = map[discovery.Query]discovery.Discover{}

// DiscoveryFunctionLookup finds a discovery function registration for a given query.
func DiscoveryFunctionLookup(q discovery.Query) (discovery.Discover, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	df, ok := discoveryFunctions[q]
	return df, ok
}

// RegisterDiscoveryFunction registers a discovery function for a given query.
func RegisterDiscoveryFunction(q discovery.Query, discover discovery.Discover) {
	_, ok := RegisteredResourceSubtypes()[q.API]
	if !ok {
		panic(errors.Errorf("trying to register discovery function for unregistered subtype %q", q.API))
	}
	if _, ok := discoveryFunctions[q]; ok {
		panic(errors.Errorf("trying to register two discovery functions for subtype %q and model %q", q.API, q.Model))
	}
	discoveryFunctions[q] = discover
}

// FindValidServiceModels returns a list of valid models for a specified service.
func FindValidServiceModels(rName resource.Name) []resource.Model {
	validModels := make([]resource.Model, 0)
	for key := range RegisteredServices() {
		if strings.Contains(key, rName.Subtype.String()) {
			splitName := strings.Split(key, "/")
			model, err := resource.NewModelFromString(splitName[1])
			if err != nil {
				utils.UncheckedError(err)
				continue
			}
			validModels = append(validModels, model)
		}
	}
	return validModels
}

// ReconfigurableComponent is implemented when component/service of a robot is reconfigurable.
type ReconfigurableComponent interface {
	// Reconfigure reconfigures the resource
	Reconfigure(ctx context.Context, cfg config.Component, deps Dependencies) error
}

// ReconfigurableService is implemented when component/service of a robot is reconfigurable.
type ReconfigurableService interface {
	// Reconfigure reconfigures the resource
	Reconfigure(ctx context.Context, cfg config.Service, deps Dependencies) error
}
