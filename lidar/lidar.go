// Package lidar defines interfaces for working with LiDARs.
//
// It also provides a means for displaying lidar scans as images.
package lidar

import (
	"context"

	"github.com/golang/geo/r2"
)

// A Lidar represents a LiDAR that can scan an area and return metadata
// about itself. Currently it is only suitable for 2D measurements
// but can be easily expanded to 3D.
type Lidar interface {
	// Info returns metadata about the lidar.
	Info(ctx context.Context) (map[string]interface{}, error)

	// Start starts the lidar to prepare for scanning and should
	// only be called once during its lifetime unless stop is called.
	Start(ctx context.Context) error

	// Stop stops the lidar and prevents future scans; it can be called at any time.
	Stop(ctx context.Context) error

	// Scan returns measurements oriented from the pose at the moment
	// the method was called. Multiple measurements at the same angle are
	// permitted to be returned.
	Scan(ctx context.Context, options ScanOptions) (Measurements, error)

	// Range returns the maximum distance in millimeters that the lidar can
	// reliably report measurements at.
	Range(ctx context.Context) (float64, error)

	// Bounds returns a two-dimensional bounding box of where measurements will
	// fall in represented in millimeters.
	Bounds(ctx context.Context) (r2.Point, error)

	// AngularResolution reports the minimum distance in degrees that the lidar
	// can produce measurements for.
	AngularResolution(ctx context.Context) (float64, error)
}

// Type identifies the type of lidar. These are typically registered in
// via RegisterType.
type Type string

// Some builtin device types.
const (
	TypeUnknown = "unknown"
	TypeFake    = "fake"
)

// ScanOptions modify how scan results will be produced and are subject
// to interpretation by the lidar implementation.
type ScanOptions struct {
	// Count determines how many scans to perform.
	Count int

	// NoFilter hints to the implementation to give as raw results as possible.
	// Normally an implementation may do some extra work to eliminate false
	// positives but this can be expensive and undesired.
	NoFilter bool
}
