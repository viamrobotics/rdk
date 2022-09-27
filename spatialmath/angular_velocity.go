package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
)

// AngularVelocity contains angular velocity in deg/s across x/y/z axes.
type AngularVelocity r3.Vector

// OrientationToAngularVel calculates an angular velocity based on an orientation change over a time differnce.
func OrientationToAngularVel(diff Orientation, dt float64) AngularVelocity {
	return EulerToAngVel(*diff.EulerAngles(), dt)
}

// EulerToAngVel calculates an angular velocity based on an orientation change expressed in euler angles over a time differnce.
func EulerToAngVel(diffEu EulerAngles, dt float64) AngularVelocity {
	return AngularVelocity{
		X: diffEu.Roll/dt - math.Sin(diffEu.Pitch)*diffEu.Yaw/dt,
		Y: math.Cos(diffEu.Roll)*diffEu.Pitch/dt + math.Cos(diffEu.Pitch)*math.Sin(diffEu.Roll)*diffEu.Yaw/dt,
		Z: -math.Sin(diffEu.Roll)*diffEu.Pitch/dt + math.Cos(diffEu.Pitch)*math.Cos(diffEu.Roll)*diffEu.Yaw/dt,
	}
}

// MulAngVel scales the angular velocity by a single scalar value.
func (av *AngularVelocity) MulAngVel(t float64) AngularVelocity {
	return AngularVelocity{
		X: t * av.X,
		Y: t * av.Y,
		Z: t * av.Z,
	}
}

// R3ToAngVel converts an r3Vector to an angular velocity.
func R3ToAngVel(vec r3.Vector) *AngularVelocity {
	return &AngularVelocity{X: vec.X, Y: vec.Y, Z: vec.Z}
}

// PointAngVel returns the angular velocity using the defiition
// r X v / |r|.
func PointAngVel(r, v r3.Vector) AngularVelocity {
	r.Normalize()
	return AngularVelocity(r.Cross(v))
}
