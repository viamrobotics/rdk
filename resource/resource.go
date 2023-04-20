/*
Package resource contains types that help identify and classify resources (components/services) of a robot.
The three most imporant types in this package are: Subtype (which represents an API for a resource), Model (which represents a specific
implementation of an API), and Name (which represents a specific instantiation of a resource.)

Both Subtype and Model have a "triplet" format which begins with a namespace. Subtype has "namespace:type:subtype" with "type" in this
case being either "service" or "component." Model has "namespace:modelfamily:modelname" with "modelfamily" being somewhat arbitrary,
and useful mostly for organization/grouping. Note that each "tier" contains the tier to the left it. Such that ModelFamily contains
Namespace, and Model itself contains ModelFamily.

An example resource (say, a motor) may use the motor API, and thus have the Subtype "rdk:component:motor" and have a model such as
"rdk:builtin:gpio". Each individual instance of that motor will have an arbitrary name (defined in the robot's configuration) and that
is represented by a Name type, which also includes the Subtype and (optionally) the remote it belongs to. Thus, the Name contains
everything (API, remote info, and unique name) to locate and cast a resource to the correct interface when requested by a client. While
Model is typically only needed during resource instantiation.
*/
package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

// define a few typed strings.
type (
	// Namespace identifies the namespaces robot resources can live in.
	Namespace string
	// TypeName identifies the resource types that robot resources can be.
	TypeName string
	// SubtypeName identifies the resources subtypes that robot resources can be.
	SubtypeName string
	// RemoteName identifies the remote the resource is attached to.
	RemoteName string
)

// Placeholder definitions for a few known constants.
const (
	ResourceNamespaceRDK  = Namespace("rdk")
	ResourceTypeComponent = TypeName("component")
	ResourceTypeService   = TypeName("service")
	DefaultServiceName    = "builtin"
	DefaultMaxInstance    = 1

	// NamespaceRDKInternal is the namespace to use for internal services.
	NamespaceRDKInternal = Namespace("rdk-internal")
)

var (
	reservedChars         = [...]string{":", "+"} // colons are delimiters for remote names, plus signs are used for WebRTC track names.
	resRegexValidator     = regexp.MustCompile(`^([\w-]+:[\w-]+:(?:[\w-]+))\/?([\w-]+:(?:[\w-]+:)*)?(.+)?$`)
	subtypeRegexValidator = regexp.MustCompile(`^([\w-]+):([\w-]+):([\w-]+)$`)
	// DefaultServiceModel is used for builtin services.
	DefaultServiceModel = NewDefaultModel("builtin")
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
		return nil, DependencyNotFoundError(name)
	}
	return res, nil
}

// Type represents a known component/service type of a robot.
type Type struct {
	Namespace    Namespace `json:"namespace"`
	ResourceType TypeName  `json:"type"`
}

// NewType creates a new Type based on parameters passed in.
func NewType(namespace Namespace, rType TypeName) Type {
	return Type{namespace, rType}
}

// Validate ensures that important fields exist and are valid.
func (t Type) Validate() error {
	if t.Namespace == "" {
		return errors.New("namespace field for resource missing or invalid")
	}
	if t.ResourceType == "" {
		return errors.New("type field for resource missing or invalid")
	}
	if err := ContainsReservedCharacter(string(t.Namespace)); err != nil {
		return err
	}
	if err := ContainsReservedCharacter(string(t.ResourceType)); err != nil {
		return err
	}
	if !singleFieldRegexValidator.MatchString(string(t.Namespace)) {
		return errors.Errorf("string %q is not a valid type namespace", t.Namespace)
	}
	if !singleFieldRegexValidator.MatchString(string(t.ResourceType)) {
		return errors.Errorf("string %q is not a valid type name", t.ResourceType)
	}
	return nil
}

// String returns the resource type string for the component.
func (t Type) String() string {
	return fmt.Sprintf("%s:%s", t.Namespace, t.ResourceType)
}

// Subtype represents a known component/service (resource) API.
type Subtype struct {
	Type
	ResourceSubtype SubtypeName `json:"subtype"`
}

// An RPCSubtype provides RPC information about a particular subtype.
type RPCSubtype struct {
	Subtype      Subtype
	ProtoSvcName string
	Desc         *desc.ServiceDescriptor
}

// NewSubtype creates a new Subtype based on parameters passed in.
func NewSubtype(namespace Namespace, rType TypeName, subtype SubtypeName) Subtype {
	resourceType := NewType(namespace, rType)
	return Subtype{resourceType, subtype}
}

// NewDefaultSubtype creates a new Subtype based on parameters passed in.
func NewDefaultSubtype(subtype SubtypeName, rType TypeName) Subtype {
	resourceType := NewType(ResourceNamespaceRDK, rType)
	return Subtype{resourceType, subtype}
}

// Validate ensures that important fields exist and are valid.
func (s Subtype) Validate() error {
	if err := s.Type.Validate(); err != nil {
		return err
	}
	if s.ResourceSubtype == "" {
		return errors.New("subtype field for resource missing or invalid")
	}
	if err := ContainsReservedCharacter(string(s.ResourceSubtype)); err != nil {
		return err
	}
	if !singleFieldRegexValidator.MatchString(string(s.ResourceSubtype)) {
		return errors.Errorf("string %q is not a valid subtype name", s.ResourceSubtype)
	}
	return nil
}

// String returns the resource subtype string for the component.
func (s Subtype) String() string {
	return fmt.Sprintf("%s:%s", s.Type, s.ResourceSubtype)
}

// UnmarshalJSON parses namespace:type:subtype strings to the full Subtype struct.
func (s *Subtype) UnmarshalJSON(data []byte) error {
	stStr := strings.Trim(string(data), "\"'")
	if subtypeRegexValidator.MatchString(stStr) {
		matches := subtypeRegexValidator.FindStringSubmatch(stStr)
		s.Namespace = Namespace(matches[1])
		s.ResourceType = TypeName(matches[2])
		s.ResourceSubtype = SubtypeName(matches[3])
		return nil
	}

	var tempSt map[string]string
	if err := json.Unmarshal(data, &tempSt); err != nil {
		return err
	}

	s.Namespace = Namespace(tempSt["namespace"])
	s.ResourceType = TypeName(tempSt["type"])
	s.ResourceSubtype = SubtypeName(tempSt["subtype"])

	return s.Validate()
}

// Name represents a known component/service representation of a robot.
type Name struct {
	Subtype
	Remote RemoteName
	Name   string
}

// UnmarshalJSON unmarshals a resource name from a string.
func (n *Name) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	newN, err := NewFromString(s)
	if err != nil {
		return err
	}
	*n = newN
	return nil
}

// NewName creates a new Name based on parameters passed in.
func NewName(namespace Namespace, rType TypeName, subtype SubtypeName, name string) Name {
	resourceSubtype := NewSubtype(namespace, rType, subtype)
	r := strings.Split(name, ":")
	remote := RemoteName(strings.Join(r[0:len(r)-1], ":"))
	nameIdent := r[len(r)-1]
	return Name{
		Subtype: resourceSubtype,
		Name:    nameIdent,
		Remote:  remote,
	}
}

// newRemoteName creates a new Name for a resource attached to a remote.
func newRemoteName(remote RemoteName, namespace Namespace, rType TypeName, subtype SubtypeName, name string) Name {
	n := NewName(namespace, rType, subtype, name)
	n.Remote = remote
	return n
}

// NameFromSubtype creates a new Name based on a Subtype and name string passed in.
func NameFromSubtype(subtype Subtype, name string) Name {
	return NewName(subtype.Namespace, subtype.ResourceType, subtype.ResourceSubtype, name)
}

// NewFromString creates a new Name based on a fully qualified resource name string passed in.
func NewFromString(resourceName string) (Name, error) {
	if !resRegexValidator.MatchString(resourceName) {
		return Name{}, errors.Errorf("string %q is not a valid resource name", resourceName)
	}
	matches := resRegexValidator.FindStringSubmatch(resourceName)
	rSubtypeParts := strings.Split(matches[1], ":")
	remote := matches[2]
	if len(remote) > 0 {
		remote = remote[:len(remote)-1]
	}
	return newRemoteName(RemoteName(remote), Namespace(rSubtypeParts[0]),
		TypeName(rSubtypeParts[1]), SubtypeName(rSubtypeParts[2]), matches[3]), nil
}

// NewSubtypeFromString creates a new Subtype from string like: %s:%s:%s.
func NewSubtypeFromString(subtypeName string) (Subtype, error) {
	if !subtypeRegexValidator.MatchString(subtypeName) {
		return Subtype{}, errors.Errorf("string %q is not a valid subtype name", subtypeName)
	}
	matches := subtypeRegexValidator.FindStringSubmatch(subtypeName)
	return NewSubtype(Namespace(matches[1]), TypeName(matches[2]), SubtypeName(matches[3])), nil
}

// PrependRemote returns a Name with a remote prepended.
func (n Name) PrependRemote(remote RemoteName) Name {
	if len(n.Remote) > 0 {
		remote = RemoteName(strings.Join([]string{string(remote), string(n.Remote)}, ":"))
	}
	return newRemoteName(
		remote,
		n.Namespace,
		n.ResourceType,
		n.ResourceSubtype,
		n.Name)
}

// PopRemote pop the first remote from a Name (if any) and returns the new Name.
func (n Name) PopRemote() Name {
	if n.Remote == "" {
		return n
	}
	remotes := strings.Split(string(n.Remote), ":")
	return newRemoteName(
		RemoteName(strings.Join(remotes[1:], ":")),
		n.Namespace,
		n.ResourceType,
		n.ResourceSubtype,
		n.Name)
}

// ContainsRemoteNames return true if the resource is a remote resource.
func (n Name) ContainsRemoteNames() bool {
	return len(n.Remote) > 0
}

// RemoveRemoteName returns a new name with remote removed.
func RemoveRemoteName(n Name) Name {
	tempName := NameFromSubtype(n.Subtype, n.Name)
	tempName.Remote = ""
	return tempName
}

// ShortName returns the short name on Name n in the form of <remote>:<name>.
func (n Name) ShortName() string {
	nameR := n.Name
	if n.Remote != "" {
		nameR = fmt.Sprintf("%s:%s", n.Remote, nameR)
	}
	return nameR
}

// ShortNameForClient returns the short name to be used in client calls (e.g. remotes)
// assuming a remote is already in the name.
func (n Name) ShortNameForClient() string {
	return n.PopRemote().ShortName()
}

// Validate ensures that important fields exist and are valid.
func (n Name) Validate() error {
	if n.Name == "" {
		return errors.New("name field for resource is empty")
	}
	if err := n.Subtype.Validate(); err != nil {
		return err
	}
	if err := ContainsReservedCharacter(n.Name); err != nil {
		return err
	}
	return nil
}

// String returns the fully qualified name for the resource.
func (n Name) String() string {
	name := n.Subtype.String()
	if n.Remote != "" {
		name = fmt.Sprintf("%s/%s:%s", name, n.Remote, n.Name)
	} else {
		name = fmt.Sprintf("%s/%s", name, n.Name)
	}
	return name
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

// AsNamed is a helper to let this name return itself as a basic resource that does
// nothing.
func (n Name) AsNamed() Named {
	return selfNamed{n}
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
func NewCloseOnlyResource(name Name, closeFunc utils.ContextCloserFunc) Resource {
	return &closeOnlyResource{Named: name.AsNamed(), closeFunc: closeFunc}
}

func (r *closeOnlyResource) Close(ctx context.Context) error {
	return r.closeFunc(ctx)
}
