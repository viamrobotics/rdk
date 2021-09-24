package spatialmath

import (
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"
)

// Pose represents a 6dof pose, position and orientation, with respect to the origin.
// The Point() method returns the position in (x,y,z) mm coordinates,
// and the Orientation() method returns an Orientation object, which has methods to parametrize
// the rotation in multiple different representations.
type Pose interface {
	Point() r3.Vector
	Orientation() Orientation
}

// NewZeroPose returns a pose at (0,0,0) with same orientation as whatever frame it is placed in.
func NewZeroPose() Pose {
	return newdualQuaternion()
}

// NewPoseFromOrientation takes in a position and orientation and returns a Pose.
func NewPoseFromOrientation(point r3.Vector, o Orientation) Pose {
	return NewPoseFromOrientationVector(point, o.OrientationVectorRadians())
}

// NewPoseFromOrientationVector takes in a position and orientation vector and returns a Pose.
func NewPoseFromOrientationVector(point r3.Vector, ov *OrientationVec) Pose {
	quat := newdualQuaternion()
	if ov != nil {
		quat = newdualQuaternionFromRotation(ov)
	}
	quat.SetTranslation(point.X, point.Y, point.Z)
	return quat
}

// NewPoseFromAxisAngle takes in a position, rotationAxis, and angle and returns a Pose.
// angle is input in radians.
func NewPoseFromAxisAngle(point, rotationAxis r3.Vector, angle float64) Pose {
	emptyVec := r3.Vector{0, 0, 0}
	if rotationAxis == emptyVec || angle == 0 {
		return newdualQuaternion()
	}
	aa := R4AA{Theta: angle, RX: rotationAxis.X, RY: rotationAxis.Y, RZ: rotationAxis.Z}

	quat := newdualQuaternion()
	quat.Real = aa.ToQuat()
	quat.SetTranslation(point.X, point.Y, point.Z)
	return quat
}

// NewPoseFromPoint takes in a cartesian (x,y,z) and stores it as a vector.
// It will have the same orientation as the frame it is in.
func NewPoseFromPoint(point r3.Vector) Pose {
	quat := newdualQuaternion()
	quat.SetTranslation(point.X, point.Y, point.Z)
	return quat
}

// NewPoseFromArmPos creates a new pose from an arm position
func NewPoseFromArmPos(pos *pb.ArmPosition) Pose {
	return newdualQuaternionFromArmPos(pos)
}

// NewPoseFromDH creates a new pose from denavit hartenberg parameters.
func NewPoseFromDH(a, d, alpha float64) Pose {
	return newdualQuaternionFromDH(a, d, alpha)
}

// Compose treats Poses as functions A(x) and B(x), and produces a new function C(x) = A(B(x)).
// It converts the poses to dual quaternions and multiplies them together, normalizes the transformm and returns a new Pose.
// Composition does not commute in general, i.e. you cannot guarantee ABx == BAx
func Compose(a, b Pose) Pose {
	aq := newdualQuaternionFromPose(a)
	bq := newdualQuaternionFromPose(b)
	result := newdualQuaternion()
	result.Number = aq.Transformation(bq.Number)

	// Normalization
	if vecLen := quat.Abs(result.Real); vecLen != 1 {
		result.Real = quat.Scale(1/vecLen, result.Real)
	}
	return result
}

// PoseDelta returns the difference between two dualQuaternion.
// We use quaternion/angle axis for this because distances are well-defined.
func PoseDelta(a, b Pose) []float64 {
	ret := make([]float64, 6)

	aQ := a.Orientation().Quaternion()
	bQ := b.Orientation().Quaternion()

	quatBetween := quat.Mul(bQ, quat.Conj(aQ))

	otherTrans := b.Point()
	mTrans := a.Point()
	aa := QuatToR3AA(quatBetween)
	zero := R3AA{1, 0, 0}
	if aa == zero {
		aa.RX = 0
	}
	ret[0] = otherTrans.X - mTrans.X
	ret[1] = otherTrans.Y - mTrans.Y
	ret[2] = otherTrans.Z - mTrans.Z
	ret[3] = aa.RX
	ret[4] = aa.RY
	ret[5] = aa.RZ
	return ret
}

// PoseToArmPos converts a pose to an arm position
func PoseToArmPos(p Pose) *pb.ArmPosition {
	final := &pb.ArmPosition{}
	pt := p.Point()
	final.X = pt.X
	final.Y = pt.Y
	final.Z = pt.Z
	poseOV := p.Orientation().OrientationVectorRadians()
	final.Theta = utils.RadToDeg(poseOV.Theta)
	final.OX = poseOV.OX
	final.OY = poseOV.OY
	final.OZ = poseOV.OZ
	return final
}

// Invert will return the inverse of a pose. So if a given pose p is the pose of A relative to B, Invert(p) will give
// the pose of B relative to A
func Invert(p Pose) Pose {
	return newdualQuaternionFromPose(p).Invert()
}
