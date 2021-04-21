package sensor

import (
	"context"
)

type Device interface {
	Readings(ctx context.Context) ([]interface{}, error)
	Desc() DeviceDescription
}

type DeviceType string

type DeviceDescription struct {
	Type DeviceType
	Path string
}
