// Package resource contains a Resource type that can be used to hold information about a robot component or service.
package resource

import (
	"context"
	"fmt"
	"strings"

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
)

// Placeholder definitions for a few known constants.
const (
	ResourceNamespaceRDK  = Namespace("rdk")
	ResourceTypeComponent = TypeName("component")
	ResourceTypeService   = TypeName("service")
)

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
	return nil
}

// String returns the resource subtype string for the component.
func (s Subtype) String() string {
	return fmt.Sprintf("%s:%s", s.Type, s.ResourceSubtype)
}

// Name represents a known component/service representation of a robot.
type Name struct {
	Subtype
	Name string
}

// NewName creates a new Name based on parameters passed in.
func NewName(namespace Namespace, rType TypeName, subtype SubtypeName, name string) Name {
	isService := rType == ResourceTypeService
	resourceSubtype := NewSubtype(namespace, rType, subtype)
	nameIdent := name
	if isService {
		nameIdent = ""
	}
	return Name{
		Subtype: resourceSubtype,
		Name:    nameIdent,
	}
}

// NameFromSubtype creates a new Name based on a Subtype and name string passed in.
func NameFromSubtype(subtype Subtype, name string) Name {
	return NewName(subtype.Namespace, subtype.ResourceType, subtype.ResourceSubtype, name)
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
	return NewName(Namespace(rSubtypeParts[0]), TypeName(rSubtypeParts[1]), SubtypeName(rSubtypeParts[2]), name), nil
}

// Validate ensures that important fields exist and are valid.
func (n Name) Validate() error {
	return n.Subtype.Validate()
}

// String returns the fully qualified name for the resource.
func (n Name) String() string {
	if n.Name == "" || (n.ResourceType == ResourceTypeService) {
		return n.Subtype.String()
	}
	return fmt.Sprintf("%s/%s", n.Subtype, n.Name)
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
