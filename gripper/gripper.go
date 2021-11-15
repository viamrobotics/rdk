// Package gripper defines a robotic gripper.
package gripper

import (
	"context"
)

// A Gripper represents a physical robotic gripper.
type Gripper interface {
	// Open opens the gripper.
	Open(ctx context.Context) error

	// Grab makes the gripper grab.
	// returns true if we grabbed something.
	Grab(ctx context.Context) (bool, error)
}
