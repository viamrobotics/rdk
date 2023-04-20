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
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

type (
	// A CreateResource creates a resource (component/service) from a collection of dependencies and a given config.
	CreateResource[T resource.Resource] func(
		ctx context.Context,
		deps resource.Dependencies,
		conf resource.Config,
		logger golog.Logger,
	) (T, error)

	// A DeprecatedCreateResourceWithRobot creates a resource from a robot and a given config.
	DeprecatedCreateResourceWithRobot[T resource.Resource] func(
		ctx context.Context,
		r robot.Robot,
		conf resource.Config,
		logger golog.Logger,
	) (T, error)

	// CreateStatus creates a status from a given resource. The return type is expected to be comprised of string keys
	// (or it should be possible to decompose it into string keys) and values comprised of primitives, list of primitives,
	// maps with string keys (or at least can be decomposed into one), or lists of the aforementioned type of maps.
	// Results with other types of data are not guaranteed.
	CreateStatus[T resource.Resource] func(ctx context.Context, res T) (interface{}, error)

	// A CreateRPCClient will create the client for the resource.
	CreateRPCClient[T resource.Resource] func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (T, error)
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

// A Resource stores a construction info for a resource (component/service). A single constructor is mandatory.
type Resource[T resource.Resource] struct {
	Constructor CreateResource[T]
	// TODO(RSDK-418): remove this legacy constructor once all resources that use it no longer need to receive the entire robot.
	DeprecatedRobotConstructor DeprecatedCreateResourceWithRobot[T]
	// Not for public use yet; currently experimental
	WeakDependencies []internal.ResourceMatcher
}

// ResourceSubtype stores subtype-specific functions and clients.
type ResourceSubtype[T resource.Resource] struct {
	Status                      CreateStatus[T]
	RPCServiceServerConstructor func(subtypeColl resource.SubtypeCollection[T]) interface{}
	RPCServiceHandler           rpc.RegisterServiceHandlerFromEndpointFunc
	RPCServiceDesc              *grpc.ServiceDesc
	ReflectRPCServiceDesc       *desc.ServiceDescriptor `copy:"shallow"`
	RPCClient                   CreateRPCClient[T]

	// MaxInstance sets a limit on the number of this subtype allowed on a robot.
	// If MaxInstance is not set then it will default to 0 and there will be no limit.
	MaxInstance int

	MakeEmptyCollection func() resource.SubtypeCollection[resource.Resource]

	typedVersion interface{} // the registry guarantees the type safety here
}

// RegisterRPCService registers this subtype into the given RPC server.
func (rs ResourceSubtype[T]) RegisterRPCService(
	ctx context.Context,
	rpcServer rpc.Server,
	subtypeColl resource.SubtypeCollection[T],
) error {
	if rs.RPCServiceServerConstructor == nil {
		return nil
	}
	return rpcServer.RegisterServiceServer(
		ctx,
		rs.RPCServiceDesc,
		rs.RPCServiceServerConstructor(subtypeColl),
		rs.RPCServiceHandler,
	)
}

// SubtypeGrpc stores functions necessary for a resource subtype to be accessible through grpc.
type SubtypeGrpc struct{}

// all registries.
var (
	registryMu       sync.RWMutex
	resourceRegistry = map[string]Resource[resource.Resource]{}
	subtypeRegistry  = map[resource.Subtype]ResourceSubtype[resource.Resource]{}
)

// RegisterService registers a model for a service and its construction info. It's a helper for
// RegisterResource.
func RegisterService[T resource.Resource](subtype resource.Subtype, model resource.Model, res Resource[T]) {
	if subtype.ResourceType != resource.ResourceTypeService {
		panic(errors.Errorf("trying to register a non-service subtype: %q, model: %q", subtype, model))
	}
	RegisterResource(subtype, model, res)
}

// RegisterComponent registers a model for a component and its construction info. It's a helper for
// RegisterResource.
func RegisterComponent[T resource.Resource](subtype resource.Subtype, model resource.Model, res Resource[T]) {
	if subtype.ResourceType != resource.ResourceTypeComponent {
		panic(errors.Errorf("trying to register a non-component subtype: %q, model: %q", subtype, model))
	}
	RegisterResource(subtype, model, res)
}

// RegisterResource registers a model for a resource (component/service) with and its construction info.
func RegisterResource[T resource.Resource](subtype resource.Subtype, model resource.Model, res Resource[T]) {
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
	resourceRegistry[qName] = makeGenericResource(res)
}

// makeGenericResource allows a registration to be generic and ensures all input/output types
// are actually T's.
func makeGenericResource[T resource.Resource](typed Resource[T]) Resource[resource.Resource] {
	reg := Resource[resource.Resource]{
		WeakDependencies: typed.WeakDependencies,
	}
	if typed.Constructor != nil {
		reg.Constructor = func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return typed.Constructor(ctx, deps, conf, logger)
		}
	}
	if typed.DeprecatedRobotConstructor != nil {
		reg.DeprecatedRobotConstructor = func(
			ctx context.Context,
			r robot.Robot,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return typed.DeprecatedRobotConstructor(ctx, r, conf, logger)
		}
	}

	return reg
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
func ResourceLookup(subtype resource.Subtype, model resource.Model) (Resource[resource.Resource], bool) {
	qName := fmt.Sprintf("%s/%s", subtype, model)

	if registration, ok := RegisteredResources()[qName]; ok {
		return registration, true
	}
	return Resource[resource.Resource]{}, false
}

// RegisterResourceSubtype register a ResourceSubtype to its corresponding resource subtype.
func RegisterResourceSubtype[T resource.Resource](subtype resource.Subtype, creator ResourceSubtype[T]) {
	registryMu.Lock()
	defer registryMu.Unlock()
	_, old := subtypeRegistry[subtype]
	if old {
		panic(errors.Errorf("trying to register two of the same resource subtype: %s", subtype))
	}
	if creator.RPCServiceServerConstructor != nil &&
		(creator.RPCServiceDesc == nil || creator.RPCServiceHandler == nil) {
		panic(errors.Errorf("cannot register a RPC enabled subtype with no RPC service description or handler: %s", subtype))
	}

	if creator.RPCServiceDesc != nil && creator.ReflectRPCServiceDesc == nil {
		reflectSvcDesc, err := grpcreflect.LoadServiceDescriptor(creator.RPCServiceDesc)
		if err != nil {
			panic(err)
		}
		creator.ReflectRPCServiceDesc = reflectSvcDesc
	}
	subtypeRegistry[subtype] = makeGenericResourceSubtype(subtype, creator)
}

// genericSubypeCollection wraps a typed collection so that it can be used generically. It ensures
// types going in are typed to T.
type genericSubypeCollection[T resource.Resource] struct {
	typed resource.SubtypeCollection[T]
}

func (g genericSubypeCollection[T]) Resource(name string) (resource.Resource, error) {
	return g.typed.Resource(name)
}

func (g genericSubypeCollection[T]) ReplaceAll(resources map[resource.Name]resource.Resource) error {
	if len(resources) == 0 {
		return nil
	}
	copied := make(map[resource.Name]T, len(resources))
	for k, v := range resources {
		typed, err := resource.AsType[T](v)
		if err != nil {
			return err
		}
		copied[k] = typed
	}
	return g.typed.ReplaceAll(copied)
}

func (g genericSubypeCollection[T]) Add(resName resource.Name, res resource.Resource) error {
	typed, err := resource.AsType[T](res)
	if err != nil {
		return err
	}
	return g.typed.Add(resName, typed)
}

func (g genericSubypeCollection[T]) Remove(name resource.Name) error {
	return g.typed.Remove(name)
}

func (g genericSubypeCollection[T]) ReplaceOne(resName resource.Name, res resource.Resource) error {
	typed, err := resource.AsType[T](res)
	if err != nil {
		return err
	}
	return g.typed.ReplaceOne(resName, typed)
}

// makeGenericResourceSubtype allows a registration to be generic and ensures all input/output types
// are actually T's.
func makeGenericResourceSubtype[T resource.Resource](
	subtype resource.Subtype,
	typed ResourceSubtype[T],
) ResourceSubtype[resource.Resource] {
	reg := ResourceSubtype[resource.Resource]{
		RPCServiceDesc:        typed.RPCServiceDesc,
		RPCServiceHandler:     typed.RPCServiceHandler,
		ReflectRPCServiceDesc: typed.ReflectRPCServiceDesc,
		MaxInstance:           typed.MaxInstance,
		typedVersion:          typed,
		MakeEmptyCollection: func() resource.SubtypeCollection[resource.Resource] {
			return genericSubypeCollection[T]{resource.NewEmptySubtypeCollection[T](subtype)}
		},
	}
	if typed.Status != nil {
		reg.Status = func(ctx context.Context, res resource.Resource) (interface{}, error) {
			typedRes, err := resource.AsType[T](res)
			if err != nil {
				return nil, err
			}
			return typed.Status(ctx, typedRes)
		}
	}
	if typed.RPCServiceServerConstructor != nil {
		reg.RPCServiceServerConstructor = func(
			coll resource.SubtypeCollection[resource.Resource],
		) interface{} {
			// it will always be this type since we are the only ones who can make
			// a generic resource subtype registration.
			genericColl, err := utils.AssertType[genericSubypeCollection[T]](coll)
			if err != nil {
				return err
			}
			return typed.RPCServiceServerConstructor(genericColl.typed)
		}
	}
	if typed.RPCClient != nil {
		reg.RPCClient = func(
			ctx context.Context,
			conn rpc.ClientConn,
			name resource.Name,
			logger golog.Logger,
		) (resource.Resource, error) {
			return typed.RPCClient(ctx, conn, name, logger)
		}
	}

	return reg
}

// DeregisterResourceSubtype removes a previously registered subtype.
func DeregisterResourceSubtype(subtype resource.Subtype) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(subtypeRegistry, subtype)
}

// GenericResourceSubtypeLookup looks up a ResourceSubtype by the given subtype. false is returned if
// there is none.
func GenericResourceSubtypeLookup(subtype resource.Subtype) (ResourceSubtype[resource.Resource], bool) {
	if registration, ok := RegisteredResourceSubtypes()[subtype]; ok {
		return registration, true
	}
	return ResourceSubtype[resource.Resource]{}, false
}

// ResourceSubtypeLookup looks up a ResourceSubtype by the given subtype. false is returned if
// there is none or error if an error occurs.
func ResourceSubtypeLookup[T resource.Resource](subtype resource.Subtype) (ResourceSubtype[T], bool, error) {
	var zero ResourceSubtype[T]
	if registration, ok := RegisteredResourceSubtypes()[subtype]; ok {
		typed, err := utils.AssertType[ResourceSubtype[T]](registration.typedVersion)
		if err != nil {
			return zero, false, err
		}
		return typed, true, nil
	}
	return zero, false, nil
}

// RegisteredResources returns a copy of the registered resources.
func RegisteredResources() map[string]Resource[resource.Resource] {
	registryMu.RLock()
	defer registryMu.RUnlock()
	copied, err := copystructure.Copy(resourceRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Resource[resource.Resource])
}

// RegisteredResourceSubtypes returns a copy of the registered resource subtypes.
func RegisteredResourceSubtypes() map[resource.Subtype]ResourceSubtype[resource.Resource] {
	registryMu.RLock()
	defer registryMu.RUnlock()
	toCopy := make(map[resource.Subtype]ResourceSubtype[resource.Resource], len(subtypeRegistry))
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

// StatusFunc adapts the given typed status function to an untyped value.
func StatusFunc[T resource.Resource, U proto.Message](
	f func(ctx context.Context, res T) (U, error),
) func(ctx context.Context, res T) (any, error) {
	return func(ctx context.Context, res T) (any, error) {
		return f(ctx, res)
	}
}
