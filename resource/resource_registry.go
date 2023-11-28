package resource

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

type (
	// An APIModel is the tuple that identifies a model implementing an API.
	APIModel struct {
		API   API
		Model Model
	}

	// A Create creates a resource (component/service) from a collection of dependencies and a given config.
	Create[ResourceT Resource] func(
		ctx context.Context,
		deps Dependencies,
		conf Config,
		logger logging.Logger,
	) (ResourceT, error)

	// A DeprecatedCreateWithRobot creates a resource from a robot and a given config.
	DeprecatedCreateWithRobot[ResourceT Resource] func(
		ctx context.Context,
		// Must be converted later. we do not pass the robot due to a package cycle. It's kludgy
		// but it's deprecated :).
		r any,
		conf Config,
		logger logging.Logger,
	) (ResourceT, error)

	// CreateStatus creates a status from a given resource. The return type is expected to be comprised of string keys
	// (or it should be possible to decompose it into string keys) and values comprised of primitives, list of primitives,
	// maps with string keys (or at least can be decomposed into one), or lists of the aforementioned type of maps.
	// Results with other types of data are not guaranteed.
	CreateStatus[ResourceT Resource] func(ctx context.Context, res ResourceT) (interface{}, error)

	// A CreateRPCClient will create the client for the resource.
	CreateRPCClient[ResourceT Resource] func(
		ctx context.Context,
		conn rpc.ClientConn,
		remoteName string,
		name Name,
		logger logging.Logger,
	) (ResourceT, error)

	// An AttributeMapConverter converts an attribute map into a native config type for a resource.
	AttributeMapConverter[ConfigT any] func(attributes utils.AttributeMap) (ConfigT, error)

	// LinkAssocationConfig allows one resource to associate a specific association config
	// to its own config. This is generally done by a specific resource (e.g. data capture of many components).
	LinkAssocationConfig[ConfigT any] func(conf ConfigT, resAssociation interface{}) error
)

// A DependencyNotReadyError is used whenever we reference a dependency that has not been
// constructed and registered yet.
type DependencyNotReadyError struct {
	Name   string
	Reason error
}

func (e *DependencyNotReadyError) Error() string {
	return fmt.Sprintf("dependency %q is not ready yet; reason=%s", e.Name, e.Reason)
}

// PrettyPrint returns a formatted string representing a `DependencyNotReadyError` error. This can be useful as a
// `DependencyNotReadyError` often wraps a series of lower level `DependencyNotReadyError` errors.
func (e *DependencyNotReadyError) PrettyPrint() string {
	var leafError error
	indent := ""
	ret := strings.Builder{}
	// Iterate through each `Reason`, incrementing the indent at each level.
	for curError := e; curError != nil; indent = fmt.Sprintf("%v%v", indent, "  ") {
		// Give the top-level error different language.
		if curError == e {
			ret.WriteString(fmt.Sprintf("Dependency %q is not ready yet\n", curError.Name))
		} else {
			ret.WriteString(indent)
			ret.WriteString(fmt.Sprintf("- Because %q is not ready yet\n", curError.Name))
		}

		// If the `Reason` is also of type `DependencyNotReadyError`, we keep going with the
		// "because X is not ready" language. The leaf error will be framed separately.
		var errArt *DependencyNotReadyError
		if errors.As(curError.Reason, &errArt) {
			curError = errArt
		} else {
			leafError = curError.Reason
			curError = nil
		}
	}

	ret.WriteString(indent)
	ret.WriteString(fmt.Sprintf("- Because %q", leafError))

	return ret.String()
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

	// WeakDependencies is a list of Matchers that find resources on the robot that fit the criteria they are looking for
	// and register them as dependencies on the resource being registered.
	// NOTE: This is currently an experimental feature and subject to change.
	WeakDependencies []Matcher

	// Discover looks around for information about this specific model.
	Discover DiscoveryFunc

	// configType can be used to dynamically inspect the resource config type.
	configType reflect.Type

	api       API
	isDefault bool
}

// ConfigReflectType returns the reflective resource config type.
func (r Registration[ResourceT, ConfigT]) ConfigReflectType() reflect.Type {
	return r.configType
}

// APIRegistration stores api-specific functions and clients.
type APIRegistration[ResourceT Resource] struct {
	Status                      CreateStatus[ResourceT]
	RPCServiceServerConstructor func(apiColl APIResourceCollection[ResourceT]) interface{}
	RPCServiceHandler           rpc.RegisterServiceHandlerFromEndpointFunc
	RPCServiceDesc              *grpc.ServiceDesc
	ReflectRPCServiceDesc       *desc.ServiceDescriptor
	RPCClient                   CreateRPCClient[ResourceT]

	// MaxInstance sets a limit on the number of this api allowed on a robot.
	// If MaxInstance is not set then it will default to 0 and there will be no limit.
	MaxInstance int

	MakeEmptyCollection func() APIResourceCollection[Resource]

	typedVersion interface{} // the registry guarantees the type safety here
}

// RegisterRPCService registers this api into the given RPC server.
func (rs APIRegistration[ResourceT]) RegisterRPCService(
	ctx context.Context,
	rpcServer rpc.Server,
	apiColl APIResourceCollection[ResourceT],
) error {
	if rs.RPCServiceServerConstructor == nil {
		return nil
	}
	return rpcServer.RegisterServiceServer(
		ctx,
		rs.RPCServiceDesc,
		rs.RPCServiceServerConstructor(apiColl),
		rs.RPCServiceHandler,
	)
}

// AssociatedNameUpdater allows an associated config to have its names updated externally.
type AssociatedNameUpdater interface {
	UpdateResourceNames(func(n Name) Name)
}

// An AssociatedConfigRegistration describes how to convert all attributes
// for a type of resource associated with another resource (e.g. data capture on a resource).
type AssociatedConfigRegistration[AssocT AssociatedNameUpdater] struct {
	// AttributeMapConverter is used to convert raw attributes to the resource's native associated config.
	AttributeMapConverter AttributeMapConverter[AssocT]

	api API
}

// all registries.
var (
	registryMu                    sync.RWMutex
	registry                      = map[APIModel]Registration[Resource, ConfigValidator]{}
	apiRegistry                   = map[API]APIRegistration[Resource]{}
	associatedConfigRegistrations = []AssociatedConfigRegistration[AssociatedNameUpdater]{}
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
		defaults = append(defaults, NewName(reg.api, DefaultServiceName))
	}
	return defaults
}

// RegisterService registers a model for a service and its construction info. It's a helper for
// Register.
func RegisterService[ResourceT Resource, ConfigT ConfigValidator](api API, model Model, reg Registration[ResourceT, ConfigT]) {
	if !api.IsService() {
		panic(errors.Errorf("trying to register a non-service api: %q, model: %q", api, model))
	}
	Register(api, model, reg)
}

// RegisterDefaultService registers a default model for a service and its construction info. It's a helper for
// RegisterService.
func RegisterDefaultService[ResourceT Resource, ConfigT ConfigValidator](
	api API,
	model Model,
	reg Registration[ResourceT, ConfigT],
) {
	if !api.IsService() {
		panic(errors.Errorf("trying to register a non-service api: %q, model: %q", api, model))
	}
	reg.isDefault = true
	Register(api, model, reg)
}

// RegisterComponent registers a model for a component and its construction info. It's a helper for
// Register.
func RegisterComponent[ResourceT Resource, ConfigT ConfigValidator](
	api API,
	model Model,
	reg Registration[ResourceT, ConfigT],
) {
	if !api.IsComponent() {
		panic(errors.Errorf("trying to register a non-component api: %q, model: %q", api, model))
	}
	Register(api, model, reg)
}

// Register registers a model for a resource (component/service) with and its construction info.
func Register[ResourceT Resource, ConfigT ConfigValidator](
	api API,
	model Model,
	reg Registration[ResourceT, ConfigT],
) {
	registryMu.Lock()
	defer registryMu.Unlock()

	apiModel := APIModel{api, model}
	_, old := registry[apiModel]
	if old {
		panic(errors.Errorf("trying to register two resources with same api: %q, model: %q", api, model))
	}
	if reg.Constructor == nil && reg.DeprecatedRobotConstructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for api: %q, model: %q", api, model))
	}
	if reg.Constructor != nil && reg.DeprecatedRobotConstructor != nil {
		panic(errors.Errorf("can only register one kind of constructor for api: %q, model: %q", api, model))
	}
	var zero ConfigT
	zeroT := reflect.TypeOf(zero)
	if reg.AttributeMapConverter == nil {
		if zeroT != nil && zeroT != noNativeConfigType {
			// provide one for free
			reg.AttributeMapConverter = TransformAttributeMap[ConfigT]
		}
	}
	reg.api = api
	reg.configType = zeroT
	registry[apiModel] = makeGenericResourceRegistration(reg)
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
		api:              typed.api,
		configType:       typed.configType,
	}
	if typed.Constructor != nil {
		reg.Constructor = func(
			ctx context.Context,
			deps Dependencies,
			conf Config,
			logger logging.Logger,
		) (Resource, error) {
			return typed.Constructor(ctx, deps, conf, logger)
		}
	}
	if typed.DeprecatedRobotConstructor != nil {
		reg.DeprecatedRobotConstructor = func(
			ctx context.Context,
			r any,
			conf Config,
			logger logging.Logger,
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
func Deregister(api API, model Model) {
	registryMu.Lock()
	defer registryMu.Unlock()
	apiModel := APIModel{api, model}
	delete(registry, apiModel)
}

// LookupRegistration looks up a creator by the given api and model. nil is returned if
// there is no creator registered.
func LookupRegistration(api API, model Model) (Registration[Resource, ConfigValidator], bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	apiModel := APIModel{api, model}
	if registration, ok := registry[apiModel]; ok {
		return registration, true
	}
	return Registration[Resource, ConfigValidator]{}, false
}

// RegisterAPI register a ResourceAPI to its corresponding resource api.
func RegisterAPI[ResourceT Resource](api API, creator APIRegistration[ResourceT]) {
	registryMu.Lock()
	defer registryMu.Unlock()
	_, old := apiRegistry[api]
	if old {
		panic(errors.Errorf("trying to register two of the same resource api: %s", api))
	}
	if creator.RPCServiceServerConstructor != nil &&
		(creator.RPCServiceDesc == nil || creator.RPCServiceHandler == nil) {
		panic(errors.Errorf("cannot register a RPC enabled api with no RPC service description or handler: %s", api))
	}

	if creator.RPCServiceDesc != nil && creator.ReflectRPCServiceDesc == nil {
		reflectSvcDesc, err := grpcreflect.LoadServiceDescriptor(creator.RPCServiceDesc)
		if err != nil {
			panic(err)
		}
		creator.ReflectRPCServiceDesc = reflectSvcDesc
	}
	apiRegistry[api] = makeGenericAPIRegistration(api, creator)
}

// RegisterAPIWithAssociation register a ResourceAPI to its corresponding resource api
// along with a way to allow other resources to associate into its config.
func RegisterAPIWithAssociation[ResourceT Resource, AssocT AssociatedNameUpdater](
	api API,
	creator APIRegistration[ResourceT],
	association AssociatedConfigRegistration[AssocT],
) {
	RegisterAPI(api, creator)
	association.api = api
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

// LookupAssociatedConfigRegistration finds the resource association config registration for the given api.
func LookupAssociatedConfigRegistration(api API) (AssociatedConfigRegistration[AssociatedNameUpdater], bool) {
	for _, conv := range associatedConfigRegistrations {
		if conv.api == api {
			return conv, true
		}
	}
	return AssociatedConfigRegistration[AssociatedNameUpdater]{}, false
}

// makeGenericAssociatedConfigRegistration allows an association to be generic and ensures all input/output types
// are actually T's.
func makeGenericAssociatedConfigRegistration[AssocT AssociatedNameUpdater](
	typed AssociatedConfigRegistration[AssocT],
) AssociatedConfigRegistration[AssociatedNameUpdater] {
	reg := AssociatedConfigRegistration[AssociatedNameUpdater]{
		// NOTE: any fields added to AssociatedConfigRegistration must be copied/adapted here.
		api: typed.api,
	}
	if typed.AttributeMapConverter != nil {
		reg.AttributeMapConverter = func(attributes utils.AttributeMap) (AssociatedNameUpdater, error) {
			return typed.AttributeMapConverter(attributes)
		}
	}

	return reg
}

// genericSubypeCollection wraps a typed collection so that it can be used generically. It ensures
// types going in are typed to T.
type genericSubypeCollection[ResourceT Resource] struct {
	typed APIResourceCollection[ResourceT]
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

// makeGenericResourceRegistrationAPI allows a registration to be generic and ensures all input/output types
// are actually T's.
func makeGenericAPIRegistration[ResourceT Resource](
	api API,
	typed APIRegistration[ResourceT],
) APIRegistration[Resource] {
	reg := APIRegistration[Resource]{
		// NOTE: any fields added to APIRegistration must be copied/adapted here.
		RPCServiceDesc:        typed.RPCServiceDesc,
		RPCServiceHandler:     typed.RPCServiceHandler,
		ReflectRPCServiceDesc: typed.ReflectRPCServiceDesc,
		MaxInstance:           typed.MaxInstance,
		typedVersion:          typed,
		MakeEmptyCollection: func() APIResourceCollection[Resource] {
			return genericSubypeCollection[ResourceT]{NewEmptyAPIResourceCollection[ResourceT](api)}
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
			coll APIResourceCollection[Resource],
		) interface{} {
			// it will always be this type since we are the only ones who can make
			// a generic resource api registration.
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
			remoteName string,
			name Name,
			logger logging.Logger,
		) (Resource, error) {
			return typed.RPCClient(ctx, conn, remoteName, name, logger)
		}
	}

	return reg
}

// DeregisterAPI removes a previously registered api.
func DeregisterAPI(api API) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(apiRegistry, api)
}

// LookupGenericAPIRegistration looks up a ResourceAPI by the given api. false is returned if
// there is none.
func LookupGenericAPIRegistration(api API) (APIRegistration[Resource], bool) {
	if registration, ok := RegisteredAPIs()[api]; ok {
		return registration, true
	}
	return APIRegistration[Resource]{}, false
}

// LookupAPIRegistration looks up a ResourceAPI by the given api. false is returned if
// there is none or error if an error occurs.
func LookupAPIRegistration[ResourceT Resource](api API) (APIRegistration[ResourceT], bool, error) {
	var zero APIRegistration[ResourceT]
	if registration, ok := RegisteredAPIs()[api]; ok {
		typed, err := utils.AssertType[APIRegistration[ResourceT]](registration.typedVersion)
		if err != nil {
			return zero, false, err
		}
		return typed, true, nil
	}
	return zero, false, nil
}

// RegisteredAPIs returns a copy of the registered resource apis.
func RegisteredAPIs() map[API]APIRegistration[Resource] {
	registryMu.RLock()
	defer registryMu.RUnlock()
	toCopy := make(map[API]APIRegistration[Resource], len(apiRegistry))
	for k, v := range apiRegistry {
		toCopy[k] = v
	}
	return toCopy
}

// RegisteredResources returns a copy of the registered resources.
func RegisteredResources() map[APIModel]Registration[Resource, ConfigValidator] {
	registryMu.RLock()
	defer registryMu.RUnlock()
	toCopy := make(map[APIModel]Registration[Resource, ConfigValidator], len(registry))
	for k, v := range registry {
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
