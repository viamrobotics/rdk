package lidar

import (
	"context"
	"image"
)

type Device interface {
	Info(ctx context.Context) (map[string]interface{}, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	// assumes the device is in a fixed point for the duration
	// of the scan
	Scan(ctx context.Context, options ScanOptions) (Measurements, error)
	Range(ctx context.Context) (int, error)
	Bounds(ctx context.Context) (image.Point, error)
	AngularResolution(ctx context.Context) (float64, error)
}

type DeviceType string

const (
	DeviceTypeUnknown = "unknown"
	DeviceTypeFake    = "fake"
)

type ScanOptions struct {
	Count    int
	NoFilter bool
}
