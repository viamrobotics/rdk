package api

import "context"

type Gripper interface {
	Open(ctx context.Context) error
	Grab(ctx context.Context) (bool, error)

	Close(ctx context.Context) error // closes the connection, not the gripper
}
