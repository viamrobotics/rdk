// Package imu defines the interface of an IMU providing angular velocity, roll,
// pitch, and yaw measurements.
package imu

import (
	"context"

	"go.viam.com/core/resource"
	"go.viam.com/core/spatialmath"
)

// SubtypeName is a constant that identifies the component resource subtype string "arm"
const SubtypeName = resource.SubtypeName("imu")

// Subtype is a constant that identifies the component resource subtype
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named IMU's typed resource name
func Named(name string) resource.Name {
	return resource.NewFromSubtype(Subtype, name)
}

// An IMU represents a sensor that can report AngularVelocity and Orientation measurements.
type IMU interface {
	sensor.Sensor
	AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error)
	Orientation(ctx context.Context) (spatialmath.Orientation, error)
}
