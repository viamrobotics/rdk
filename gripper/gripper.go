// Package gripper defines a robotic gripper.
package gripper

import (
	"context"

	"go.viam.com/core/resource"
)

// A Gripper represents a physical robotic gripper.
type Gripper interface {
	// Open opens the gripper.
	Open(ctx context.Context) error

	// Grab makes the gripper grab.
	// returns true if we grabbed something.
	Grab(ctx context.Context) (bool, error)
}

// Subtype is a constant that identifies the component resource subtype
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	resource.ResourceSubtypeGripper,
)

// Named is a helper for getting the named grippers's typed resource name
func Named(name string) resource.Name {
	return resource.NewFromSubtype(Subtype, name)
}
