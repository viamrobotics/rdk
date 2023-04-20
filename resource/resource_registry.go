package resource

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

	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/utils"
)

type (
	// A Create creates a resource (component/service) from a collection of dependencies and a given config.
	Create[T Resource] func(
		ctx context.Context,
		deps Dependencies,
		conf Config,
		logger golog.Logger,
	) (T, error)

	// A DeprecatedCreateWithRobot creates a resource from a robot and a given config.
	DeprecatedCreateWithRobot[T Resource] func(
		ctx context.Context,
		// Must be converted later. we do not pass the robot due to a package cycle. It's kludgy
		// but it's deprecated :).
		r any,
		conf Config,
		logger golog.Logger,
	) (T, error)

	// CreateStatus creates a status from a given resource. The return type is expected to be comprised of string keys
	// (or it should be possible to decompose it into string keys) and values comprised of primitives, list of primitives,
	// maps with string keys (or at least can be decomposed into one), or lists of the aforementioned type of maps.
	// Results with other types of data are not guaranteed.
	CreateStatus[T Resource] func(ctx context.Context, res T) (interface{}, error)

	// A CreateRPCClient will create the client for the resource.
	CreateRPCClient[T Resource] func(ctx context.Context, conn rpc.ClientConn, name Name, logger golog.Logger) (T, error)
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

// A Registration stores construction info for a resource (component/service). A single constructor is mandatory.
type Registration[T Resource, U any] struct {
	Constructor Create[T]
	// TODO(RSDK-418): remove this legacy constructor once all resources that use it no longer need to receive the entire robot.
	DeprecatedRobotConstructor DeprecatedCreateWithRobot[T]
	// Not for public use yet; currently experimental
	WeakDependencies []internal.ResourceMatcher

	AttributeMapConverter func(attributes utils.AttributeMap) (U, error)
}

// A RegistrationWithConfig stores construction info for a resource (component/service). A single constructor is mandatory.
type RegistrationWithConfig[T Resource, U any] Registration[T, U]

// SubtypeRegistration stores subtype-specific functions and clients.
type SubtypeRegistration[T Resource] struct {
	Status                      CreateStatus[T]
	RPCServiceServerConstructor func(subtypeColl SubtypeCollection[T]) interface{}
	RPCServiceHandler           rpc.RegisterServiceHandlerFromEndpointFunc
	RPCServiceDesc              *grpc.ServiceDesc
	ReflectRPCServiceDesc       *desc.ServiceDescriptor `copy:"shallow"`
	RPCClient                   CreateRPCClient[T]

	// MaxInstance sets a limit on the number of this subtype allowed on a robot.
	// If MaxInstance is not set then it will default to 0 and there will be no limit.
	MaxInstance int

	MakeEmptyCollection func() SubtypeCollection[Resource]

	typedVersion interface{} // the registry guarantees the type safety here
}

// RegisterRPCService registers this subtype into the given RPC server.
func (rs SubtypeRegistration[T]) RegisterRPCService(
	ctx context.Context,
	rpcServer rpc.Server,
	subtypeColl SubtypeCollection[T],
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
	registryMu      sync.RWMutex
	registry        = map[string]Registration[Resource, any]{}
	subtypeRegistry = map[Subtype]SubtypeRegistration[Resource]{}
)

// RegisterService registers a model for a service and its construction info. It's a helper for
// Register.
func RegisterService[T Resource, U any](subtype Subtype, model Model, res Registration[T, U]) {
	if subtype.ResourceType != ResourceTypeService {
		panic(errors.Errorf("trying to register a non-service subtype: %q, model: %q", subtype, model))
	}
	Register(subtype, model, res)
}

// RegisterComponent registers a model for a component and its construction info. It's a helper for
// Register.
func RegisterComponent[T Resource, U any](subtype Subtype, model Model, res Registration[T, U]) {
	if subtype.ResourceType != ResourceTypeComponent {
		panic(errors.Errorf("trying to register a non-component subtype: %q, model: %q", subtype, model))
	}
	Register(subtype, model, res)
}

// Register registers a model for a resource (component/service) with and its construction info.
func Register[T Resource, U any](subtype Subtype, model Model, reg Registration[T, U]) {
	registryMu.Lock()
	defer registryMu.Unlock()
	qName := fmt.Sprintf("%s/%s", subtype.String(), model.String())

	_, old := registry[qName]
	if old {
		panic(errors.Errorf("trying to register two resources with same subtype: %q, model: %q", subtype, model))
	}
	if reg.Constructor == nil && reg.DeprecatedRobotConstructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype: %q, model: %q", subtype, model))
	}
	if reg.Constructor != nil && reg.DeprecatedRobotConstructor != nil {
		panic(errors.Errorf("can only register one kind of constructor for subtype: %q, model: %q", subtype, model))
	}
	registry[qName] = makeGenericResourceRegistration(reg)
}

// makeGenericResourceRegistration allows a registration to be generic and ensures all input/output types
// are actually T's.
func makeGenericResourceRegistration[T Resource, U any](typed Registration[T, U]) Registration[Resource, any] {
	reg := Registration[Resource, any]{
		WeakDependencies: typed.WeakDependencies,
	}
	if typed.Constructor != nil {
		reg.Constructor = func(
			ctx context.Context,
			deps Dependencies,
			conf Config,
			logger golog.Logger,
		) (Resource, error) {
			return typed.Constructor(ctx, deps, conf, logger)
		}
	}
	if typed.DeprecatedRobotConstructor != nil {
		reg.DeprecatedRobotConstructor = func(
			ctx context.Context,
			r any,
			conf Config,
			logger golog.Logger,
		) (Resource, error) {
			return typed.DeprecatedRobotConstructor(ctx, r, conf, logger)
		}
	}
	if typed.AttributeMapConverter != nil {
		reg.AttributeMapConverter = func(attributes utils.AttributeMap) (any, error) {
			return typed.AttributeMapConverter(attributes)
		}
	}

	return reg
}

// Deregister removes a previously registered resource.
func Deregister(subtype Subtype, model Model) {
	registryMu.Lock()
	defer registryMu.Unlock()
	qName := fmt.Sprintf("%s/%s", subtype, model)
	delete(registry, qName)
}

// LookupRegistration looks up a creator by the given subtype and model. nil is returned if
// there is no creator registered.
func LookupRegistration(subtype Subtype, model Model) (Registration[Resource, any], bool) {
	qName := fmt.Sprintf("%s/%s", subtype, model)

	if registration, ok := RegisteredResources()[qName]; ok {
		return registration, true
	}
	return Registration[Resource, any]{}, false
}

// RegisterSubtype register a ResourceSubtype to its corresponding resource subtype.
func RegisterSubtype[T Resource](subtype Subtype, creator SubtypeRegistration[T]) {
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
	subtypeRegistry[subtype] = makeGenericSubtypeRegistration(subtype, creator)
}

// genericSubypeCollection wraps a typed collection so that it can be used generically. It ensures
// types going in are typed to T.
type genericSubypeCollection[T Resource] struct {
	typed SubtypeCollection[T]
}

func (g genericSubypeCollection[T]) Resource(name string) (Resource, error) {
	return g.typed.Resource(name)
}

func (g genericSubypeCollection[T]) ReplaceAll(resources map[Name]Resource) error {
	if len(resources) == 0 {
		return nil
	}
	copied := make(map[Name]T, len(resources))
	for k, v := range resources {
		typed, err := AsType[T](v)
		if err != nil {
			return err
		}
		copied[k] = typed
	}
	return g.typed.ReplaceAll(copied)
}

func (g genericSubypeCollection[T]) Add(resName Name, res Resource) error {
	typed, err := AsType[T](res)
	if err != nil {
		return err
	}
	return g.typed.Add(resName, typed)
}

func (g genericSubypeCollection[T]) Remove(name Name) error {
	return g.typed.Remove(name)
}

func (g genericSubypeCollection[T]) ReplaceOne(resName Name, res Resource) error {
	typed, err := AsType[T](res)
	if err != nil {
		return err
	}
	return g.typed.ReplaceOne(resName, typed)
}

// makeGenericResourceRegistrationSubtype allows a registration to be generic and ensures all input/output types
// are actually T's.
func makeGenericSubtypeRegistration[T Resource](
	subtype Subtype,
	typed SubtypeRegistration[T],
) SubtypeRegistration[Resource] {
	reg := SubtypeRegistration[Resource]{
		RPCServiceDesc:        typed.RPCServiceDesc,
		RPCServiceHandler:     typed.RPCServiceHandler,
		ReflectRPCServiceDesc: typed.ReflectRPCServiceDesc,
		MaxInstance:           typed.MaxInstance,
		typedVersion:          typed,
		MakeEmptyCollection: func() SubtypeCollection[Resource] {
			return genericSubypeCollection[T]{NewEmptySubtypeCollection[T](subtype)}
		},
	}
	if typed.Status != nil {
		reg.Status = func(ctx context.Context, res Resource) (interface{}, error) {
			typedRes, err := AsType[T](res)
			if err != nil {
				return nil, err
			}
			return typed.Status(ctx, typedRes)
		}
	}
	if typed.RPCServiceServerConstructor != nil {
		reg.RPCServiceServerConstructor = func(
			coll SubtypeCollection[Resource],
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
			name Name,
			logger golog.Logger,
		) (Resource, error) {
			return typed.RPCClient(ctx, conn, name, logger)
		}
	}

	return reg
}

// DeregisterSubtype removes a previously registered subtype.
func DeregisterSubtype(subtype Subtype) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(subtypeRegistry, subtype)
}

// LookupGenericSubtypeRegistration looks up a ResourceSubtype by the given subtype. false is returned if
// there is none.
func LookupGenericSubtypeRegistration(subtype Subtype) (SubtypeRegistration[Resource], bool) {
	if registration, ok := RegisteredSubtypes()[subtype]; ok {
		return registration, true
	}
	return SubtypeRegistration[Resource]{}, false
}

// LookupSubtypeRegistration looks up a ResourceSubtype by the given subtype. false is returned if
// there is none or error if an error occurs.
func LookupSubtypeRegistration[T Resource](subtype Subtype) (SubtypeRegistration[T], bool, error) {
	var zero SubtypeRegistration[T]
	if registration, ok := RegisteredSubtypes()[subtype]; ok {
		typed, err := utils.AssertType[SubtypeRegistration[T]](registration.typedVersion)
		if err != nil {
			return zero, false, err
		}
		return typed, true, nil
	}
	return zero, false, nil
}

// RegisteredResources returns a copy of the registered resources.
func RegisteredResources() map[string]Registration[Resource, any] {
	registryMu.RLock()
	defer registryMu.RUnlock()
	copied, err := copystructure.Copy(registry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Registration[Resource, any])
}

// RegisteredSubtypes returns a copy of the registered resource subtypes.
func RegisteredSubtypes() map[Subtype]SubtypeRegistration[Resource] {
	registryMu.RLock()
	defer registryMu.RUnlock()
	toCopy := make(map[Subtype]SubtypeRegistration[Resource], len(subtypeRegistry))
	for k, v := range subtypeRegistry {
		toCopy[k] = v
	}
	return toCopy
}

var discoveryFunctions = map[DiscoveryQuery]DiscoveryFunc{}

// LookupDiscoveryFunction finds a discovery function registration for a given query.
func LookupDiscoveryFunction(q DiscoveryQuery) (DiscoveryFunc, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	df, ok := discoveryFunctions[q]
	return df, ok
}

// RegisterDiscoveryFunction registers a discovery function for a given query.
func RegisterDiscoveryFunction(q DiscoveryQuery, discover DiscoveryFunc) {
	_, ok := RegisteredSubtypes()[q.API]
	if !ok {
		panic(errors.Errorf("trying to register discovery function for unregistered subtype %q", q.API))
	}
	if _, ok := discoveryFunctions[q]; ok {
		panic(errors.Errorf("trying to register two discovery functions for subtype %q and model %q", q.API, q.Model))
	}
	discoveryFunctions[q] = discover
}

// LookupWeakDependency looks up a resource registration's weak dependencies on other
// resources defined statically.
// TODO(erd): just get registration instead.
func LookupWeakDependency(subtype Subtype, model Model) []internal.ResourceMatcher {
	registryMu.RLock()
	defer registryMu.RUnlock()
	qName := fmt.Sprintf("%s/%s", subtype, model)
	if registration, ok := RegisteredResources()[qName]; ok {
		return registration.WeakDependencies
	}
	return nil
}

// StatusFunc adapts the given typed status function to an untyped value.
func StatusFunc[T Resource, U proto.Message](
	f func(ctx context.Context, res T) (U, error),
) func(ctx context.Context, res T) (any, error) {
	return func(ctx context.Context, res T) (any, error) {
		return f(ctx, res)
	}
}
