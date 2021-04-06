package sensor

import "context"

type Device interface {
	Readings(ctx context.Context) ([]interface{}, error)
	Close(ctx context.Context) error
}

type DeviceType string

type DeviceDescription struct {
	Type DeviceType
	Path string
}
