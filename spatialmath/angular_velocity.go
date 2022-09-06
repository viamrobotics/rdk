package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"
)

// AngularVelocity contains angular velocity in deg/s across x/y/z axes.
type AngularVelocity r3.Vector

// TODO (rh) figure out if from to orientation actually makes more sense for these functions, and if they should be
// function taking in a general orientation or specific orientations

// OrientationToAngularVel calculates an angular velocity based on an orientation change over a time differnce.
func (av *AngularVelocity) OrientationToAngularVel(o Orientation, dt float64) *AngularVelocity {
	axA := o.AxisAngles()

	return &AngularVelocity{
		X: axA.RX * axA.Theta / dt,
		Y: axA.RY * axA.Theta / dt,
		Z: axA.RZ * axA.Theta / dt,
	}
}

// QuatToAngVel calculates an angular velocity based on an orientation change expressed in quaternions over a time differnce.
func (av *AngularVelocity) QuatToAngVel(diffQ quat.Number, dt float64) *AngularVelocity {
	// todo (rh) check if normalisation needs to be performed at each step
	dqdt := quat.Number{Real: diffQ.Real / dt, Imag: diffQ.Imag / dt, Jmag: diffQ.Jmag / dt, Kmag: diffQ.Kmag / dt}
	w := quat.Scale(2, quat.Mul(quat.Conj(diffQ), dqdt))
	return &AngularVelocity{
		X: w.Imag,
		Y: w.Jmag,
		Z: w.Kmag,
	}
}

// EulerToAngVel calculates an angular velocity based on an orientation change expressed in euler angles over a time differnce.
func (av *AngularVelocity) EulerToAngVel(diffEu EulerAngles, dt float64) *AngularVelocity {
	// TODO (rh) check order for tait bryan
	return &AngularVelocity{
		X: diffEu.Roll/dt - math.Sin(diffEu.Pitch)*diffEu.Yaw/dt,
		Y: math.Cos(diffEu.Roll)*diffEu.Pitch/dt + math.Cos(diffEu.Pitch)*math.Sin(diffEu.Roll)*diffEu.Yaw/dt,
		Z: -math.Sin(diffEu.Roll)*diffEu.Pitch/dt + math.Cos(diffEu.Pitch)*math.Cos(diffEu.Roll)*diffEu.Yaw/dt,
	}
}

// RotMatToAngVel calculates an angular velocity based on an orientation change expressed in rotation matrices over a time differnce.
func (av *AngularVelocity) RotMatToAngVel(diffRm RotationMatrix, dt float64) *AngularVelocity {
	// I did this for homework once and refuse to do it again
	return av.OrientationToAngularVel(diffRm.AxisAngles(), dt)
}
