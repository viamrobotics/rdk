// Package resource contains a Resource type that can be used to hold information about a robot component or service.
package resource

import (
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
)

// Name represents a known component/service of a robot.
type Name struct {
	UUID      string
	Namespace string
	Type      string
	Subtype   string
	Name      string
}

// Validate ensures that important fields exist and are valid.
func (r Name) Validate() error {
	if _, err := uuid.Parse(r.UUID); err != nil {
		return errors.New("uuid field for resource missing or invalid")
	}
	if r.Namespace == "" {
		return errors.New("namespace field for resource missing or invalid")
	}
	if r.Type == "" {
		return errors.New("type field for resource missing or invalid")
	}
	if r.Subtype == "" {
		return errors.New("subtype field for resource missing or invalid")
	}
	return nil
}
