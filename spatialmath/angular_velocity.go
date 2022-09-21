package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"
)

// AngularVelocity contains angular velocity in deg/s across x/y/z axes.
type AngularVelocity r3.Vector

// OrientationToAngularVel calculates an angular velocity based on an orientation change over a time differnce.
func OrientationToAngularVel(diffO Orientation, dt float64) *AngularVelocity {
	axA := diffO.AxisAngles()

	return &AngularVelocity{
		X: axA.RX * axA.Theta / dt,
		Y: axA.RY * axA.Theta / dt,
		Z: axA.RZ * axA.Theta / dt,
	}
}

// QuatToAngVel calculates an angular velocity based on an orientation change expressed in quaternions over a time differnce.
func QuatToAngVel(diffQ quat.Number, dt float64) *AngularVelocity {
	ndQ := Normalize(diffQ)
	// todo (rh) check if normalisation needs to be performed at each step
	dqdt := quat.Number{Real: diffQ.Real / dt, Imag: diffQ.Imag / dt, Jmag: diffQ.Jmag / dt, Kmag: diffQ.Kmag / dt}
	ndqdt := Normalize(dqdt)
	w := quat.Scale(2, quat.Mul(quat.Conj(ndQ), ndqdt))
	return &AngularVelocity{
		X: w.Imag,
		Y: w.Jmag,
		Z: w.Kmag,
	}
}

// EulerToAngVel calculates an angular velocity based on an orientation change expressed in euler angles over a time differnce.
func EulerToAngVel(diffEu EulerAngles, dt float64) *AngularVelocity {
	return &AngularVelocity{
		X: diffEu.Roll/dt - math.Sin(diffEu.Pitch)*diffEu.Yaw/dt,
		Y: math.Cos(diffEu.Roll)*diffEu.Pitch/dt + math.Cos(diffEu.Pitch)*math.Sin(diffEu.Roll)*diffEu.Yaw/dt,
		Z: -math.Sin(diffEu.Roll)*diffEu.Pitch/dt + math.Cos(diffEu.Pitch)*math.Cos(diffEu.Roll)*diffEu.Yaw/dt,
	}
}

// RotMatToAngVel calculates an angular velocity based on an orientation change expressed in rotation matrices over a time differnce.
func RotMatToAngVel(diffRm RotationMatrix, dt float64) *AngularVelocity {
	return OrientationToAngularVel(diffRm.AxisAngles(), dt)
}

// Multiply scales the angular velocity by a single scalar value
func (av *AngularVelocity) MulAngVel(t float64) *AngularVelocity {
	return &AngularVelocity{
		X: t * av.X,
		Y: t * av.Y,
		Z: t * av.Z,
	}
}

// R3ToAngVel converts an r3Vector to an angular velocity
func R3ToAngVel(vec r3.Vector) *AngularVelocity {
	return &AngularVelocity{X: vec.X, Y: vec.Y, Z: vec.Z}
}
