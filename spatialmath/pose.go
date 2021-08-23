package spatialmath

import (
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"
)

// Pose represents a 6dof pose, position and orientation. For convenience, everything is returned as a dual quaternion.
// translation is the translation operation (Δx,Δy,Δz), in this case [1, 0, 0 ,0][0, Δx/2, Δy/2, Δz/2] is returned
// orientation is often an SO(3) matrix, in this case an orientation vector is returned
type Pose interface {
	Point() r3.Vector
	Orientation() *OrientationVec
	Invert() Pose
}

// NewEmptyPose returns a pose at (0,0,0) with same orientation as whatever frame it is placed in.
func NewEmptyPose() Pose {
	return NewDualQuaternion()
}

// NewPoseFromAxisAngle takes in a positon, rotationAxis, and angle and returns a Pose.
// angle is input in radians.
func NewPoseFromAxisAngle(point, rotationAxis r3.Vector, angle float64) Pose {
	emptyVec := r3.Vector{0, 0, 0}
	if rotationAxis == emptyVec || angle == 0 {
		return NewDualQuaternion()
	}
	aa := R4AA{Theta: angle, RX: rotationAxis.X, RY: rotationAxis.Y, RZ: rotationAxis.Z}
	
	quat := NewDualQuaternion()
	quat.Real = aa.ToQuat()
	quat.SetTranslation(point.X, point.Y, point.Z)
	return quat
}

// NewPoseFromPoint takes in a cartesian (x,y,z) and stores it as a vector.
// It will have the same orientation as the frame it is in.
func NewPoseFromPoint(point r3.Vector) Pose {
	quat := NewDualQuaternion()
	quat.SetTranslation(point.X, point.Y, point.Z)
	return quat
}

// Compose takes two poses, converts to dual quaternions and multiplies them together, then normalizes the transform.
// DualQuaternions apply their operation TO THE RIGHT. example: if you have an operation A and operation B on p
// pAB means ((pA)B). First A is applied, then B. QUATERNIONS DO NOT COMMUTE IN GENERAL! Cannot guarantee BAp == ABp!
// Note however that (pA)(B) == (p)(AB)
func Compose(a, b Pose) Pose {
	aq := NewDualQuaternionFromPose(a)
	bq := NewDualQuaternionFromPose(b)
	result := NewDualQuaternion()
	result.Number = aq.Transformation(bq.Number)

	// Normalization
	if vecLen := quat.Abs(result.Real); vecLen != 1 {
		result.Real = quat.Scale(1/vecLen, result.Real)
	}
	return result
}

// PoseDelta returns the difference between two DualQuaternion.
// We use quaternion/angle axis for this because distances are well-defined.
func PoseDelta(a, b Pose) []float64 {
	ret := make([]float64, 7)

	aQ := a.Orientation().ToQuat()
	bQ := b.Orientation().ToQuat()

	quatBetween := quat.Mul(bQ, quat.Conj(aQ))

	otherTrans := b.Point()
	mTrans := a.Point()
	aa := QuatToR4AA(quatBetween)
	ret[0] = otherTrans.X - mTrans.X
	ret[1] = otherTrans.Y - mTrans.Y
	ret[2] = otherTrans.Z - mTrans.Z
	ret[3] = aa.Theta
	ret[4] = aa.RX
	ret[5] = aa.RY
	ret[6] = aa.RZ
	return ret
}
