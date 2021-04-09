package api

import (
	"context"
)

type Gripper interface {
	Open(ctx context.Context) error
	Grab(ctx context.Context) (bool, error)
}
