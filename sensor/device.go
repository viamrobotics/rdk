package sensor

import "context"

type Device interface {
	Readings(ctx context.Context) ([]interface{}, error)
	Close(ctx context.Context) error
}
