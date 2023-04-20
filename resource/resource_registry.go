package resource

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/edaniels/golog"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/utils"
)

type (
	// A Create creates a resource (component/service) from a collection of dependencies and a given config.
	Create[ResourceT Resource] func(
		ctx context.Context,
		deps Dependencies,
		conf Config,
		logger golog.Logger,
	) (ResourceT, error)

	// A DeprecatedCreateWithRobot creates a resource from a robot and a given config.
	DeprecatedCreateWithRobot[ResourceT Resource] func(
		ctx context.Context,
		// Must be converted later. we do not pass the robot due to a package cycle. It's kludgy
		// but it's deprecated :).
		r any,
		conf Config,
		logger golog.Logger,
	) (ResourceT, error)

	// CreateStatus creates a status from a given resource. The return type is expected to be comprised of string keys
	// (or it should be possible to decompose it into string keys) and values comprised of primitives, list of primitives,
	// maps with string keys (or at least can be decomposed into one), or lists of the aforementioned type of maps.
	// Results with other types of data are not guaranteed.
	CreateStatus[ResourceT Resource] func(ctx context.Context, res ResourceT) (interface{}, error)

	// A CreateRPCClient will create the client for the resource.
	CreateRPCClient[ResourceT Resource] func(ctx context.Context, conn rpc.ClientConn, name Name, logger golog.Logger) (ResourceT, error)

	// An AttributeMapConverter converts an attribute map into a native config type for a resource.
	AttributeMapConverter[ConfigT any] func(attributes utils.AttributeMap) (ConfigT, error)

	// LinkAssocationConfig allows one resource to associate a specific association config
	// to its own config. This is generally done by a specific resource (e.g. data capture of many components).
	LinkAssocationConfig[ConfigT any] func(conf ConfigT, resAssociation interface{}) error

	// AssociationConfigWithName allows a resource to attach a name to a subtype specific
	// association config. This is generally done by the subtype registration.
	AssociationConfigWithName[AssocT any] func(resName Name, resAssociation AssocT) error
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
type Registration[ResourceT Resource, ConfigT any] struct {
	Constructor Create[ResourceT]

	// AttributeMapConverter is used to convert raw attributes to the resource's native config.
	AttributeMapConverter AttributeMapConverter[ConfigT]

	// An AssocationConfigLinker describes how to associate a
	// resource association config to a specific resource model (e.g. builtin data capture).
	AssociatedConfigLinker LinkAssocationConfig[ConfigT]

	// TODO(RSDK-418): remove this legacy constructor once all resources that use it no longer need to receive the entire robot.
	DeprecatedRobotConstructor DeprecatedCreateWithRobot[ResourceT]
	// Not for public use yet; currently experimental
	WeakDependencies []internal.ResourceMatcher

	// Discover looks around for information about this specific model.
	Discover DiscoveryFunc

	subtype   Subtype
	isDefault bool
}

// SubtypeRegistration stores subtype-specific functions and clients.
type SubtypeRegistration[ResourceT Resource] struct {
	Status                      CreateStatus[ResourceT]
	RPCServiceServerConstructor func(subtypeColl SubtypeCollection[ResourceT]) interface{}
	RPCServiceHandler           rpc.RegisterServiceHandlerFromEndpointFunc
	RPCServiceDesc              *grpc.ServiceDesc
	ReflectRPCServiceDesc       *desc.ServiceDescriptor
	RPCClient                   CreateRPCClient[ResourceT]

	// MaxInstance sets a limit on the number of this subtype allowed on a robot.
	// If MaxInstance is not set then it will default to 0 and there will be no limit.
	MaxInstance int

	MakeEmptyCollection func() SubtypeCollection[Resource]

	typedVersion interface{} // the registry guarantees the type safety here
}

// RegisterRPCService registers this subtype into the given RPC server.
func (rs SubtypeRegistration[ResourceT]) RegisterRPCService(
	ctx context.Context,
	rpcServer rpc.Server,
	subtypeColl SubtypeCollection[ResourceT],
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

// An AssociatedConfigRegistration describes how to convert all attributes
// for a type of resource associated with another resource (e.g. data capture on a resource).
type AssociatedConfigRegistration[AssocT any] struct {
	// AttributeMapConverter is used to convert raw attributes to the resource's native associated config.
	AttributeMapConverter AttributeMapConverter[AssocT]

	// WithName is used to attach a resource name to the native association config.
	WithName AssociationConfigWithName[AssocT]

	subtype Subtype
}

// all registries.
var (
	registryMu                    sync.RWMutex
	registry                      = map[string]Registration[Resource, ConfigValidator]{}
	subtypeRegistry               = map[Subtype]SubtypeRegistration[Resource]{}
	associatedConfigRegistrations = []AssociatedConfigRegistration[any]{}
)

// DefaultServices returns all servies that will be constructed by default if not
// specified in a config.
func DefaultServices() []Name {
	registryMu.RLock()
	defer registryMu.RUnlock()

	var defaults []Name
	for _, reg := range registry {
		if !reg.isDefault {
			continue
		}
		defaults = append(defaults, NameFromSubtype(reg.subtype, DefaultServiceName))
	}
	return defaults
}

// RegisterService registers a model for a service and its construction info. It's a helper for
// Register.
func RegisterService[ResourceT Resource, ConfigT ConfigValidator](subtype Subtype, model Model, reg Registration[ResourceT, ConfigT]) {
	if subtype.ResourceType != ResourceTypeService {
		panic(errors.Errorf("trying to register a non-service subtype: %q, model: %q", subtype, model))
	}
	Register(subtype, model, reg)
}

// RegisterDefaultService registers a default model for a service and its construction info. It's a helper for
// RegisterService.
func RegisterDefaultService[ResourceT Resource, ConfigT ConfigValidator](
	subtype Subtype,
	model Model,
	reg Registration[ResourceT, ConfigT],
) {
	if subtype.ResourceType != ResourceTypeService {
		panic(errors.Errorf("trying to register a non-service subtype: %q, model: %q", subtype, model))
	}
	reg.isDefault = true
	Register(subtype, model, reg)
}

// RegisterComponent registers a model for a component and its construction info. It's a helper for
// Register.
func RegisterComponent[ResourceT Resource, ConfigT ConfigValidator](
	subtype Subtype,
	model Model,
	reg Registration[ResourceT, ConfigT],
) {
	if subtype.ResourceType != ResourceTypeComponent {
		panic(errors.Errorf("trying to register a non-component subtype: %q, model: %q", subtype, model))
	}
	Register(subtype, model, reg)
}

func makeSubtypeModelString(subtype Subtype, model Model) string {
	return fmt.Sprintf("%s/%s", subtype.String(), model.String())
}

// Register registers a model for a resource (component/service) with and its construction info.
func Register[ResourceT Resource, ConfigT ConfigValidator](
	subtype Subtype,
	model Model,
	reg Registration[ResourceT, ConfigT],
) {
	registryMu.Lock()
	defer registryMu.Unlock()
	qName := makeSubtypeModelString(subtype, model)

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
	if reg.AttributeMapConverter == nil {
		var zero ConfigT
		zeroT := reflect.TypeOf(zero)
		if zeroT != nil && zeroT != noNativeConfigType {
			// provide one for free
			reg.AttributeMapConverter = TransformAttributeMap[ConfigT]
		}
	}
	reg.subtype = subtype
	registry[qName] = makeGenericResourceRegistration(reg)
}

// makeGenericResourceRegistration allows a registration to be generic and ensures all input/output types
// are actually T's.
func makeGenericResourceRegistration[ResourceT Resource, ConfigT ConfigValidator](
	typed Registration[ResourceT, ConfigT],
) Registration[Resource, ConfigValidator] {
	reg := Registration[Resource, ConfigValidator]{
		// NOTE: any fields added to Registration must be copied/adapted here.
		WeakDependencies: typed.WeakDependencies,
		Discover:         typed.Discover,
		isDefault:        typed.isDefault,
		subtype:          typed.subtype,
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
		reg.AttributeMapConverter = func(attributes utils.AttributeMap) (ConfigValidator, error) {
			return typed.AttributeMapConverter(attributes)
		}
	}
	if typed.AssociatedConfigLinker != nil {
		reg.AssociatedConfigLinker = func(conf ConfigValidator, resAssociation interface{}) error {
			typedConf, err := utils.AssertType[ConfigT](conf)
			if err != nil {
				return err
			}
			return typed.AssociatedConfigLinker(typedConf, resAssociation)
		}
	}

	return reg
}

// Deregister removes a previously registered resource.
func Deregister(subtype Subtype, model Model) {
	registryMu.Lock()
	defer registryMu.Unlock()
	qName := makeSubtypeModelString(subtype, model)
	delete(registry, qName)
}

// LookupRegistration looks up a creator by the given subtype and model. nil is returned if
// there is no creator registered.
func LookupRegistration(subtype Subtype, model Model) (Registration[Resource, ConfigValidator], bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	qName := makeSubtypeModelString(subtype, model)
	if registration, ok := registry[qName]; ok {
		return registration, true
	}
	return Registration[Resource, ConfigValidator]{}, false
}

// RegisterSubtype register a ResourceSubtype to its corresponding resource subtype.
func RegisterSubtype[ResourceT Resource](subtype Subtype, creator SubtypeRegistration[ResourceT]) {
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

// RegisterSubtypeWithAssociation register a ResourceSubtype to its corresponding resource subtype
// along with a way to allow other resources to associate into its config.
func RegisterSubtypeWithAssociation[ResourceT Resource, AssocT any](
	subtype Subtype,
	creator SubtypeRegistration[ResourceT],
	association AssociatedConfigRegistration[AssocT],
) {
	if association.WithName == nil {
		panic("must provide a WithName to a AssociatedConfigRegistration")
	}
	RegisterSubtype(subtype, creator)
	association.subtype = subtype
	if association.AttributeMapConverter == nil {
		var zero AssocT
		if reflect.TypeOf(zero) != nil {
			// provide one for free
			association.AttributeMapConverter = TransformAttributeMap[AssocT]
		}
	}
	associatedConfigRegistrations = append(
		associatedConfigRegistrations,
		makeGenericAssociatedConfigRegistration(association),
	)
}

// LookupAssociatedConfigRegistration finds the resource association config registration for the given subtype.
func LookupAssociatedConfigRegistration(subtype Subtype) (AssociatedConfigRegistration[any], bool) {
	for _, conv := range associatedConfigRegistrations {
		if conv.subtype == subtype {
			return conv, true
		}
	}
	return AssociatedConfigRegistration[any]{}, false
}

// makeGenericAssociatedConfigRegistration allows an association to be generic and ensures all input/output types
// are actually T's.
func makeGenericAssociatedConfigRegistration[AssocT any](typed AssociatedConfigRegistration[AssocT]) AssociatedConfigRegistration[any] {
	reg := AssociatedConfigRegistration[any]{
		// NOTE: any fields added to AssociatedConfigRegistration must be copied/adapted here.
		subtype: typed.subtype,
	}
	if typed.AttributeMapConverter != nil {
		reg.AttributeMapConverter = func(attributes utils.AttributeMap) (any, error) {
			return typed.AttributeMapConverter(attributes)
		}
	}
	if typed.WithName != nil {
		reg.WithName = func(resName Name, resAssociation any) error {
			typedAssoc, err := utils.AssertType[AssocT](resAssociation)
			if err != nil {
				return err
			}
			return typed.WithName(resName, typedAssoc)
		}
	}

	return reg
}

// genericSubypeCollection wraps a typed collection so that it can be used generically. It ensures
// types going in are typed to T.
type genericSubypeCollection[ResourceT Resource] struct {
	typed SubtypeCollection[ResourceT]
}

func (g genericSubypeCollection[ResourceT]) Resource(name string) (Resource, error) {
	return g.typed.Resource(name)
}

func (g genericSubypeCollection[ResourceT]) ReplaceAll(resources map[Name]Resource) error {
	if len(resources) == 0 {
		return nil
	}
	copied := make(map[Name]ResourceT, len(resources))
	for k, v := range resources {
		typed, err := AsType[ResourceT](v)
		if err != nil {
			return err
		}
		copied[k] = typed
	}
	return g.typed.ReplaceAll(copied)
}

func (g genericSubypeCollection[ResourceT]) Add(resName Name, res Resource) error {
	typed, err := AsType[ResourceT](res)
	if err != nil {
		return err
	}
	return g.typed.Add(resName, typed)
}

func (g genericSubypeCollection[ResourceT]) Remove(name Name) error {
	return g.typed.Remove(name)
}

func (g genericSubypeCollection[ResourceT]) ReplaceOne(resName Name, res Resource) error {
	typed, err := AsType[ResourceT](res)
	if err != nil {
		return err
	}
	return g.typed.ReplaceOne(resName, typed)
}

// makeGenericResourceRegistrationSubtype allows a registration to be generic and ensures all input/output types
// are actually T's.
func makeGenericSubtypeRegistration[ResourceT Resource](
	subtype Subtype,
	typed SubtypeRegistration[ResourceT],
) SubtypeRegistration[Resource] {
	reg := SubtypeRegistration[Resource]{
		// NOTE: any fields added to SubtypeRegistration must be copied/adapted here.
		RPCServiceDesc:        typed.RPCServiceDesc,
		RPCServiceHandler:     typed.RPCServiceHandler,
		ReflectRPCServiceDesc: typed.ReflectRPCServiceDesc,
		MaxInstance:           typed.MaxInstance,
		typedVersion:          typed,
		MakeEmptyCollection: func() SubtypeCollection[Resource] {
			return genericSubypeCollection[ResourceT]{NewEmptySubtypeCollection[ResourceT](subtype)}
		},
	}
	if typed.Status != nil {
		reg.Status = func(ctx context.Context, res Resource) (interface{}, error) {
			typedRes, err := AsType[ResourceT](res)
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
			genericColl, err := utils.AssertType[genericSubypeCollection[ResourceT]](coll)
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
func LookupSubtypeRegistration[ResourceT Resource](subtype Subtype) (SubtypeRegistration[ResourceT], bool, error) {
	var zero SubtypeRegistration[ResourceT]
	if registration, ok := RegisteredSubtypes()[subtype]; ok {
		typed, err := utils.AssertType[SubtypeRegistration[ResourceT]](registration.typedVersion)
		if err != nil {
			return zero, false, err
		}
		return typed, true, nil
	}
	return zero, false, nil
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

// StatusFunc adapts the given typed status function to an untyped value.
func StatusFunc[ResourceT Resource, StatusU proto.Message](
	f func(ctx context.Context, res ResourceT) (StatusU, error),
) func(ctx context.Context, res ResourceT) (any, error) {
	return func(ctx context.Context, res ResourceT) (any, error) {
		return f(ctx, res)
	}
}
