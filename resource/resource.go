/*
Package resource contains types that help identify and classify resources (components/services) of a robot.
The three most important types in this package are: API (which represents an API for a resource), Model (which represents a specific
implementation of an API), and Name (which represents a specific instantiation of a resource.)

Both API and Model have a "triplet" format that begins with a namespace. API has "namespace:type:subtype" with "type" in this
case being either "service" or "component." Model has "namespace:modelfamily:modelname" with "modelfamily" being somewhat arbitrary
and useful mostly for organization/grouping. Note that each "tier" contains the tier to the left it. Such that ModelFamily contains
Namespace, and Model itself contains ModelFamily.

An example resource (say, a motor) may use the motor API and thus have the API "rdk:component:motor" and have a model such as
"rdk:builtin:gpio". Each instance of that motor will have an arbitrary name (defined in the robot's configuration)
represented by a Name type, which also includes the API and (optionally) the remote it belongs to. Thus, the Name contains
everything (API, remote info, and unique name) to locate and cast a resource to the correct interface when requested by a client.
Model on the other hand is typically only needed during resource instantiation.
*/
package resource

import (
	"context"
	"reflect"
	"regexp"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Placeholder definitions for a few known constants.
const (
	DefaultServiceName = "builtin"
	DefaultMaxInstance = 1
)

var (
	reservedChars     = [...]string{":", "+"} // colons are delimiters for remote names, plus signs are used for WebRTC track names.
	resRegexValidator = regexp.MustCompile(`^([\w-]+:[\w-]+:(?:[\w-]+))\/?([\w-]+:(?:[\w-]+:)*)?(.+)?$`)
)

// A Resource is the fundamental building block of a robot; it is either a component or a service
// that is accessible through robot. In general, some other specific type that is the component
// or service implements this interface. All resources must know their own name and be able
// to reconfigure themselves (or signal that they must be rebuilt).
// Resources that fail to reconfigure or rebuild may be closed and must return
// errors when in a closed state for all non Close methods.
type Resource interface {
	Name() Name

	// Reconfigure must reconfigure the resource atomically and in place. If this
	// cannot be guaranteed, then usage of AlwaysRebuild or TriviallyReconfigurable
	// is permissible.
	Reconfigure(ctx context.Context, deps Dependencies, conf Config) error

	// DoCommand sends/receives arbitrary data
	DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)

	// Close must safely shut down the resource and prevent further use.
	// Close must be idempotent.
	// Later reconfiguration may allow a resource to be "open" again.
	Close(ctx context.Context) error
}

// Dependencies are a set of resources that a resource requires for reconfiguration.
type Dependencies map[Name]Resource

// FromDependencies returns a named component from a collection of dependencies.
func FromDependencies[T Resource](resources Dependencies, name Name) (T, error) {
	var zero T
	res, err := resources.Lookup(name)
	if err != nil {
		return zero, DependencyNotFoundError(name)
	}
	typedRes, ok := res.(T)
	if !ok {
		return zero, DependencyTypeError[T](name, res)
	}
	return typedRes, nil
}

// Lookup searches for a given dependency by name.
func (d Dependencies) Lookup(name Name) (Resource, error) {
	res, ok := d[name]
	if !ok {
		if !name.ContainsRemoteNames() {
			var res Resource
			// we assume the map is small and not costly to search
			for depName, depRes := range d {
				if !(depName.API == name.API && depName.Name == name.Name) {
					continue
				}
				if res != nil {
					return nil, utils.NewRemoteResourceClashError(name.Name)
				}
				res = depRes
			}
			if res != nil {
				return res, nil
			}
		}
		return nil, DependencyNotFoundError(name)
	}
	return res, nil
}

// An RPCAPI provides RPC information about a particular API.
type RPCAPI struct {
	API          API
	ProtoSvcName string
	Desc         *desc.ServiceDescriptor
}

// errReservedCharacterUsed is used when a reserved character is wrongly used in a name.
func errReservedCharacterUsed(val, reservedChar string) error {
	return errors.Errorf("reserved character %s used in name:%q", reservedChar, val)
}

// ContainsReservedCharacter returns error if string contains a reserved character.
func ContainsReservedCharacter(val string) error {
	for _, char := range reservedChars {
		if strings.Contains(val, char) {
			return errReservedCharacterUsed(val, char)
		}
	}
	return nil
}

// Actuator is any resource that can move.
type Actuator interface {
	// IsMoving returns whether the resource is moving or not
	IsMoving(context.Context) (bool, error)

	// Stop stops all movement for the resource
	Stop(context.Context, map[string]interface{}) error
}

// Shaped is any resource that can have geometries.
type Shaped interface {
	// Geometries returns the list of geometries associated with the resource, in any order. The poses of the geometries reflect their
	// current location relative to the frame of the resource.
	Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error)
}

// ErrDoUnimplemented is returned if the DoCommand methods is not implemented.
var ErrDoUnimplemented = errors.New("DoCommand unimplemented")

// TriviallyReconfigurable is to be embedded by any resource that does not care about
// changes to its config or dependencies.
type TriviallyReconfigurable struct{}

// Reconfigure always succeeds.
func (t TriviallyReconfigurable) Reconfigure(ctx context.Context, deps Dependencies, conf Config) error {
	return nil
}

// TriviallyCloseable is to be embedded by any resource that does not care about
// handling Closes. When is used, it is assumed that the resource does not need
// to return errors when furture non-Close methods are called.
type TriviallyCloseable struct{}

// Close always returns no error.
func (t TriviallyCloseable) Close(ctx context.Context) error {
	return nil
}

// TriviallyValidateConfig is to be embedded by any resource config that does not care about
// its validation or implicit dependencies; use this carefully.
type TriviallyValidateConfig struct{}

// Validate always succeeds and produces no dependencies.
func (t TriviallyValidateConfig) Validate(path string) ([]string, error) {
	return nil, nil
}

var noNativeConfigType = reflect.TypeOf(NoNativeConfig{})

// NoNativeConfig is used by types that have no significant native config.
type NoNativeConfig struct {
	TriviallyValidateConfig
}

// AlwaysRebuild is to be embedded by any resource that must always rebuild
// and not reconfigure.
type AlwaysRebuild struct{}

// Reconfigure always returns a must rebuild error.
func (a AlwaysRebuild) Reconfigure(ctx context.Context, deps Dependencies, conf Config) error {
	return NewMustRebuildError(conf.ResourceName())
}

// Named is to be embedded by any resource that just needs to return a name.
type Named interface {
	Name() Name
	DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

type selfNamed struct {
	name Name
}

// Name returns the name of the resource.
func (s selfNamed) Name() Name {
	return s.name
}

// DoCommand always returns unimplemented but can be implemented by the embedder.
func (s selfNamed) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, ErrDoUnimplemented
}

// AsType attempts to get a more specific interface from the resource.
func AsType[T Resource](from Resource) (T, error) {
	res, ok := from.(T)
	if !ok {
		var zero T
		return zero, TypeError[T](from)
	}
	return res, nil
}

type closeOnlyResource struct {
	Named
	TriviallyReconfigurable
	closeFunc func(ctx context.Context) error
}

// NewCloseOnlyResource makes a new resource that needs to be closed and
// does not need the actual resource exposed but only its close function.
func NewCloseOnlyResource(name Name, closeFunc func(ctx context.Context) error) Resource {
	return &closeOnlyResource{Named: name.AsNamed(), closeFunc: closeFunc}
}

func (r *closeOnlyResource) Close(ctx context.Context) error {
	return r.closeFunc(ctx)
}
