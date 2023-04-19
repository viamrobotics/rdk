// Package registry operates the global registry of robotic parts.
package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
)

type (
	// A CreateResource creates a resource (component/service) from a collection of dependencies and a given config.
	CreateResource func(
		ctx context.Context,
		deps resource.Dependencies,
		conf resource.Config,
		logger golog.Logger,
	) (resource.Resource, error)

	// A DeprecatedCreateResourceWithRobot creates a resource from a robot and a given config.
	DeprecatedCreateResourceWithRobot func(
		ctx context.Context,
		r robot.Robot,
		conf resource.Config,
		logger golog.Logger,
	) (resource.Resource, error)

	// CreateStatus creates a status from a given resource. The return type is expected to be comprised of string keys
	// (or it should be possible to decompose it into string keys) and values comprised of primitives, list of primitives,
	// maps with string keys (or at least can be decomposed into one), or lists of the aforementioned type of maps.
	// Results with other types of data are not guaranteed.
	CreateStatus func(ctx context.Context, resource resource.Resource) (interface{}, error)

	// A RegisterSubtypeRPCService will register the subtype service to the grpc server.
	RegisterSubtypeRPCService func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error

	// A CreateRPCClient will create the client for the resource.
	CreateRPCClient func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (resource.Resource, error)
)

// A DependencyNotReadyError is used whenever we reference a dependency that has not been
// constructed and registered yet.
type DependencyNotReadyError struct {
	Name   string
	Reason error
}

func (e *DependencyNotReadyError) Error() string {
	return fmt.Sprintf("dependency %q is not ready yet; reason=%q", e.Name, e.Reason)
}

// IsDependencyNotReadyError returns if the given error is any kind of dependency not found error.
func IsDependencyNotReadyError(err error) bool {
	var errArt *DependencyNotReadyError
	return errors.As(err, &errArt)
}

type (
	// A Resource stores a construction info for a resource (component/service). A single constructor is mandatory.
	Resource struct {
		Constructor CreateResource
		// TODO(RSDK-418): remove this legacy constructor once all resources that use it no longer need to receive the entire robot.
		DeprecatedRobotConstructor DeprecatedCreateResourceWithRobot
		// Not for public use yet; currently experimental
		WeakDependencies []internal.ResourceMatcher
	}

	// A Component is a resource with the component type (namespace:component:subtype/name).
	Component = Resource

	// A Service is a resource with the service type (namespace:service:subtype/name).
	Service = Resource
)

// ResourceSubtype stores subtype-specific functions and clients.
type ResourceSubtype struct {
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
	registryMu       sync.RWMutex
	resourceRegistry = map[string]Resource{}
	subtypeRegistry  = map[resource.Subtype]ResourceSubtype{}
)

// RegisterService registers a model for a service with and its construction info. Its a helper for
// RegisterResource.
func RegisterService(subtype resource.Subtype, model resource.Model, res Service) {
	if subtype.ResourceType != resource.ResourceTypeService {
		panic(errors.Errorf("trying to register a non-service subtype: %q, model: %q", subtype, model))
	}
	RegisterResource(subtype, model, res)
}

// RegisterComponent registers a model for a component and its construction info. It's a helper for
// RegisterResource.
func RegisterComponent(subtype resource.Subtype, model resource.Model, res Component) {
	if subtype.ResourceType != resource.ResourceTypeComponent {
		panic(errors.Errorf("trying to register a non-component subtype: %q, model: %q", subtype, model))
	}
	RegisterResource(subtype, model, res)
}

// RegisterResource registers a model for a resource (component/service) with and its construction info.
func RegisterResource(subtype resource.Subtype, model resource.Model, res Resource) {
	registryMu.Lock()
	defer registryMu.Unlock()
	qName := fmt.Sprintf("%s/%s", subtype.String(), model.String())

	_, old := resourceRegistry[qName]
	if old {
		panic(errors.Errorf("trying to register two resources with same subtype: %q, model: %q", subtype, model))
	}
	if res.Constructor == nil && res.DeprecatedRobotConstructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype: %q, model: %q", subtype, model))
	}
	if res.Constructor != nil && res.DeprecatedRobotConstructor != nil {
		panic(errors.Errorf("can only register one kind of constructor for subtype: %q, model: %q", subtype, model))
	}
	resourceRegistry[qName] = res
}

// DeregisterResource removes a previously registered resource.
func DeregisterResource(subtype resource.Subtype, model resource.Model) {
	registryMu.Lock()
	defer registryMu.Unlock()
	qName := fmt.Sprintf("%s/%s", subtype, model)
	delete(resourceRegistry, qName)
}

// ResourceLookup looks up a creator by the given subtype and model. nil is returned if
// there is no creator registered.
func ResourceLookup(subtype resource.Subtype, model resource.Model) (Resource, bool) {
	qName := fmt.Sprintf("%s/%s", subtype, model)

	if registration, ok := RegisteredResources()[qName]; ok {
		return registration, true
	}
	return Resource{}, false
}

// RegisterResourceSubtype register a ResourceSubtype to its corresponding resource subtype.
func RegisterResourceSubtype(subtype resource.Subtype, creator ResourceSubtype) {
	registryMu.Lock()
	defer registryMu.Unlock()
	_, old := subtypeRegistry[subtype]
	if old {
		panic(errors.Errorf("trying to register two of the same resource subtype: %s", subtype))
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
func ResourceSubtypeLookup(subtype resource.Subtype) (ResourceSubtype, bool) {
	if registration, ok := RegisteredResourceSubtypes()[subtype]; ok {
		return registration, true
	}
	return ResourceSubtype{}, false
}

// RegisteredResources returns a copy of the registered resources.
func RegisteredResources() map[string]Resource {
	registryMu.RLock()
	defer registryMu.RUnlock()
	copied, err := copystructure.Copy(resourceRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Resource)
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

// WeakDependencyLookup looks up a resource registration's weak dependencies on other
// resources defined statically.
func WeakDependencyLookup(subtype resource.Subtype, model resource.Model) []internal.ResourceMatcher {
	registryMu.RLock()
	defer registryMu.RUnlock()
	qName := fmt.Sprintf("%s/%s", subtype, model)
	if registration, ok := RegisteredResources()[qName]; ok {
		return registration.WeakDependencies
	}
	return nil
}
