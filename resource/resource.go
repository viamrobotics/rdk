// Package resource contains a Resource type that can be used to hold information about a robot component or service.
package resource

import (
	"fmt"
	"strings"

	"github.com/go-errors/errors"

	"github.com/google/uuid"
)

// Placeholder definitions for a few known constants
const (
	ResourceNamespaceCore   = "core"
	ResourceTypeComponent   = "component"
	ResourceTypeService     = "service"
	ResourceSubtypeArm      = "arm"
	ResourceSubtypeBase     = "base"
	ResourceSubtypeBoard    = "board"
	ResourceSubtypeCamera   = "camera"
	ResourceSubtypeFunction = "function"
	ResourceSubtypeGripper  = "gripper"
	ResourceSubtypeLidar    = "lidar"
	ResourceSubtypeMetadata = "metadata"
	ResourceSubtypeRemote   = "remote"
	ResourceSubtypeSensor   = "sensor"
	ResourceSubtypeServo    = "servo"
	ResourceSubtypeMotor    = "motor"
)

// Type represents a known component/service type of a robot.
type Type struct {
	Namespace string
	Type      string
}

// NewType creates a new Type based on parameters passed in.
func NewType(namespace string, rType string) (Type, error) {
	if namespace == "" {
		return Type{}, errors.New("namespace parameter missing or invalid")
	}
	if rType == "" {
		return Type{}, errors.New("type parameter missing or invalid")
	}
	return Type{namespace, rType}, nil
}

// Validate ensures that important fields exist and are valid.
func (t Type) Validate() error {
	if t.Namespace == "" {
		return errors.New("namespace field for resource missing or invalid")
	}
	if t.Type == "" {
		return errors.New("type field for resource missing or invalid")
	}
	return nil
}

// String returns the resource type string for the component.
func (t Type) String() string {
	return fmt.Sprintf("%s:%s", t.Namespace, t.Type)
}

// Subtype represents a known component/service subtype of a robot.
type Subtype struct {
	ResourceType Type
	Subtype      string
}

// NewSubtype creates a new Subtype based on parameters passed in.
func NewSubtype(namespace string, rType string, subtype string) (Subtype, error) {
	resourceType, err := NewType(namespace, rType)
	if err != nil {
		return Subtype{}, err
	}
	if subtype == "" {
		return Subtype{}, errors.New("subtype parameter missing or invalid")
	}
	return Subtype{resourceType, subtype}, nil
}

// Validate ensures that important fields exist and are valid.
func (s Subtype) Validate() error {
	if err := s.ResourceType.Validate(); err != nil {
		return err
	}
	if s.Subtype == "" {
		return errors.New("subtype field for resource missing or invalid")
	}
	return nil
}

// String returns the resource subtype string for the component.
func (s Subtype) String() string {
	return fmt.Sprintf("%s:%s", s.ResourceType, s.Subtype)
}

// Name represents a known component/service representation of a robot.
type Name struct {
	ResourceSubtype Subtype
	UUID            string
	Name            string
}

// NewName creates a new Name based on parameters passed in.
func NewName(namespace string, rType string, subtype string, name string) (Name, error) {
	resourceSubtype, err := NewSubtype(namespace, rType, subtype)
	if err != nil {
		return Name{}, err
	}
	i := resourceSubtype.String()
	if name != "" {
		i = fmt.Sprintf("%s/%s", i, name)
	}
	return Name{
		UUID:            uuid.NewSHA1(uuid.NameSpaceX500, []byte(i)).String(),
		ResourceSubtype: resourceSubtype,
		Name:            name,
	}, nil
}

// NewFromString creates a new Name based on a fully qualified resource name string passed in.
func NewFromString(resourceName string) (Name, error) {
	var name string
	nameParts := strings.Split(resourceName, "/")
	if len(nameParts) == 2 {
		name = nameParts[1]
	} else if len(nameParts) > 2 {
		return Name{}, errors.New("invalid resource name string: there is more than one backslash")
	}
	rSubtypeParts := strings.Split(nameParts[0], ":")
	if len(rSubtypeParts) > 3 {
		return Name{}, errors.New("invalid resource name string: there are more than 2 colons")
	}
	if len(rSubtypeParts) < 3 {
		return Name{}, errors.New("invalid resource name string: there are less than 2 colons")
	}
	return NewName(rSubtypeParts[0], rSubtypeParts[1], rSubtypeParts[2], name)
}

// Validate ensures that important fields exist and are valid.
func (n Name) Validate() error {
	if _, err := uuid.Parse(n.UUID); err != nil {
		return errors.New("uuid field for resource missing or invalid")
	}
	return n.ResourceSubtype.Validate()
}

// String returns the fully qualified name for the resource.
func (n Name) String() string {
	if n.Name == "" {
		return n.ResourceSubtype.String()
	}
	return fmt.Sprintf("%s/%s", n.ResourceSubtype, n.Name)
}

// Reconfigurable is implemented when component/service of a robot is reconfigurable.
type Reconfigurable interface {
	// Reconfigure reconfigures the resource
	Reconfigure(newResource Reconfigurable) error
}
