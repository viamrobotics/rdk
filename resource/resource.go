// Package resource contains a Resource type that can be used to hold information about a robot component or service.
package resource

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
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
)

var resRegexValidator = regexp.MustCompile(`^(rdk:\w+:(?:\w+))\/?(\w+:(?:\w+:)*)?(.+)?$`)

// Type represents a known component/service type of a robot.
type Type struct {
	Namespace    Namespace
	ResourceType TypeName
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
	if strings.Contains(string(t.Namespace), ":") {
		return errors.Errorf("resevered character : used in namespace name:%q", t.Namespace)
	}
	if strings.Contains(string(t.ResourceType), ":") {
		return errors.Errorf("resevered character : used in resourcetype name:%q", t.ResourceType)
	}
	return nil
}

// String returns the resource type string for the component.
func (t Type) String() string {
	return fmt.Sprintf("%s:%s", t.Namespace, t.ResourceType)
}

// Subtype represents a known component/service subtype of a robot.
type Subtype struct {
	Type
	ResourceSubtype SubtypeName
}

// An RPCSubtype provides RPC information about a particular subtype.
type RPCSubtype struct {
	Subtype Subtype
	Desc    *desc.ServiceDescriptor
}

// NewSubtype creates a new Subtype based on parameters passed in.
func NewSubtype(namespace Namespace, rType TypeName, subtype SubtypeName) Subtype {
	resourceType := NewType(namespace, rType)
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
	if strings.Contains(string(s.ResourceSubtype), ":") {
		return errors.Errorf("resevered character : used in resource subtype name:%q", s.ResourceSubtype)
	}
	return nil
}

// String returns the resource subtype string for the component.
func (s Subtype) String() string {
	return fmt.Sprintf("%s:%s", s.Type, s.ResourceSubtype)
}

// Name represents a known component/service representation of a robot.
type Name struct {
	Subtype
	Remote RemoteName
	Name   string
}

// NewName creates a new Name based on parameters passed in.
func NewName(namespace Namespace, rType TypeName, subtype SubtypeName, name string) Name {
	isService := rType == ResourceTypeService
	resourceSubtype := NewSubtype(namespace, rType, subtype)
	r := strings.Split(name, ":")
	remote := RemoteName(strings.Join(r[0:len(r)-1], ":"))
	nameIdent := r[len(r)-1]
	if isService {
		nameIdent = ""
	}
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

// ShortName returns the short name on Name n in the form of <remote>:<name>.
func (n Name) ShortName() string {
	nameR := n.Name
	if n.Remote != "" {
		nameR = fmt.Sprintf("%s:%s", n.Remote, nameR)
	}
	return nameR
}

// Validate ensures that important fields exist and are valid.
func (n Name) Validate() error {
	if err := n.Subtype.Validate(); err != nil {
		return err
	}
	if strings.Contains(n.Name, ":") {
		return errors.Errorf("resevered character : used in resource name:%q", n.Name)
	}
	return nil
}

// String returns the fully qualified name for the resource.
func (n Name) String() string {
	name := n.Subtype.String()
	if n.Remote != "" {
		name = fmt.Sprintf("%s/%s:", name, n.Remote)
	}
	if n.Name != "" && (n.ResourceType != ResourceTypeService) {
		if n.Remote != "" {
			name = fmt.Sprintf("%s%s", name, n.Name)
		} else {
			name = fmt.Sprintf("%s/%s", name, n.Name)
		}
	}
	return name
}

// Reconfigurable is implemented when component/service of a robot is reconfigurable.
type Reconfigurable interface {
	// Reconfigure reconfigures the resource
	Reconfigure(ctx context.Context, newResource Reconfigurable) error
}

// Updateable is implemented when component/service of a robot should be updated after the robot reconfiguration process is done.
type Updateable interface {
	// Update updates the resource
	Update(context.Context, map[Name]interface{}) error
}

// MovingCheckable is implemented when a resource of a robot returns whether it is moving or not.
type MovingCheckable interface {
	// IsMoving returns whether the resource is moving or not
	IsMoving(context.Context) (bool, error)
}

// Stoppable is implemented when a resource of a robot can stop its movement.
type Stoppable interface {
	// Stop stops all movement for the resource
	Stop(context.Context, map[string]interface{}) error
}

// OldStoppable will be deprecated soon. See Stoppable.
// TODO[RSDK-328].
type OldStoppable interface {
	// Stop stops all movement for the resource
	Stop(context.Context) error
}
