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

func (av *AngularVelocity) QuatToAngVel(q quat.Number, dt float64) *AngularVelocity {
	// todo (rh) check if normalisation needs to be performed at each step
	dqdt := quat.Number{Real: q.Real / dt, Imag: q.Imag / dt, Jmag: q.Jmag / dt, Kmag: q.Kmag / dt}
	w := quat.Scale(2, quat.Mul(quat.Conj(q), dqdt))
	return &AngularVelocity{
		X: w.Imag,
		Y: w.Jmag,
		Z: w.Kmag,
	}
}

func (av *AngularVelocity) EulerToAngVel(eu EulerAngles, dt float64) *AngularVelocity {
	// TODO (rh) check order for tait bryan
	return &AngularVelocity{
		X: eu.Roll/dt - math.Sin(eu.Pitch)*eu.Yaw/dt,
		Y: math.Cos(eu.Roll)*eu.Pitch/dt + math.Cos(eu.Pitch)*math.Sin(eu.Roll)*eu.Yaw/dt,
		Z: -math.Sin(eu.Roll)*eu.Pitch/dt + math.Cos(eu.Pitch)*math.Cos(eu.Roll)*eu.Yaw/dt,
	}

}

func (av *AngularVelocity) RotMatToAngVel(rm RotationMatrix, dt float64) *AngularVelocity {
	// I did this for homework once and refuse to do it again
	return av.OrientationToAngularVel(rm.AxisAngles(), dt)
}
