// Package spatialmath defines spatial mathematical operations.
// Poses represent a position in 6 degrees of freedom, i.e. a position and an orientation.
// Positions are represented as r3 Vectors, while Orientations are an interface able to be represented
// many different ways. This package provides various Orientation implementations as well as the ability to perform
// a variety of useful operations on Poses and Orientations.
package spatialmath

import (
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

// defaultDistanceEpsilon represents the acceptable discrepancy between two floats
// representing spatial coordinates wherein the coordinates should be
// considered equivalent.
const defaultDistanceEpsilon = 1e-8

// Pose represents a 6dof pose, position and orientation, with respect to the origin.
// The Point() method returns the position in (x,y,z) mm coordinates,
// and the Orientation() method returns an Orientation object, which has methods to parametrize
// the rotation in multiple different representations.
type Pose interface {
	Point() r3.Vector
	Orientation() Orientation
}

// PoseMap encodes the orientation interface to something serializable and human readable.
func PoseMap(p Pose) (map[string]interface{}, error) {
	oc, err := NewOrientationConfig(p.Orientation().AxisAngles())
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"point":       p.Point(),
		"orientation": oc,
	}, nil
}

// NewZeroPose returns a pose at (0,0,0) with same orientation as whatever frame it is placed in.
func NewZeroPose() Pose {
	return newDualQuaternion()
}

// NewPose takes in a position and orientation and returns a Pose.
func NewPose(p r3.Vector, o Orientation) Pose {
	if o == nil {
		return NewPoseFromPoint(p)
	}
	q := newDualQuaternion()
	q.Real = o.Quaternion()
	q.SetTranslation(p)
	return q
}

// NewPoseFromPoint takes in a cartesian (x,y,z) and stores it as a vector.
// It will have the same orientation as the frame it is in reference to.
func NewPoseFromPoint(point r3.Vector) Pose {
	q := newDualQuaternion()
	q.SetTranslation(point)
	return q
}

// NewPoseFromOrientation takes in an orientation and returns a Pose.
// It will have the same position as the frame it is in reference to.
func NewPoseFromOrientation(o Orientation) Pose {
	q := newDualQuaternion()
	q.Real = o.Quaternion()
	return q
}

// NewPoseFromProtobuf creates a new pose from a protobuf pose.
func NewPoseFromProtobuf(pos *commonpb.Pose) Pose {
	return newDualQuaternionFromProtobuf(pos)
}

// NewPoseFromDH creates a new pose from denavit hartenberg parameters.
func NewPoseFromDH(a, d, alpha float64) Pose {
	return newDualQuaternionFromDH(a, d, alpha)
}

// Compose treats Poses as functions A(x) and B(x), and produces a new function C(x) = A(B(x)).
// It converts the poses to dual quaternions and multiplies them together, normalizes the transform and returns a new Pose.
// Composition does not commute in general, i.e. you cannot guarantee ABx == BAx.
func Compose(a, b Pose) Pose {
	result := &dualQuaternion{dualQuaternionFromPose(a).Transformation(dualQuaternionFromPose(b).Number)}

	// Normalization
	if vecLen := 1 / quat.Abs(result.Real); vecLen != 1 {
		result.Real.Real *= vecLen
		result.Real.Imag *= vecLen
		result.Real.Jmag *= vecLen
		result.Real.Kmag *= vecLen
	}
	return result
}

// PoseBetween returns the difference between two dualQuaternions, that is, the dq which if multiplied by one will give the other.
func PoseBetween(a, b Pose) Pose {
	return &dualQuaternion{dualquat.Mul(dualQuaternionFromPose(b).Number, dualquat.ConjQuat(dualQuaternionFromPose(a).Number))}
}

// PoseBetweenInverse returns an origin pose which when composed with the first parameter, yields the second.
// Example: if PoseBetweenInverse(a, b) = c, then Compose(c, a) = b
// PoseBetweenInverse(a, b) is equivalent to Compose(b, PoseInverse(a)).
func PoseBetweenInverse(a, b Pose) Pose {
	result := &dualQuaternion{dualQuaternionFromPose(b).Transformation(dualquat.ConjQuat(dualQuaternionFromPose(a).Number))}
	// Normalization
	if vecLen := 1 / quat.Abs(result.Real); vecLen != 1 {
		result.Real.Real *= vecLen
		result.Real.Imag *= vecLen
		result.Real.Jmag *= vecLen
		result.Real.Kmag *= vecLen
	}
	return result
}

// PoseDelta returns the difference between two dualQuaternion.
// We use quaternion/angle axis for this because distances are well-defined.
func PoseDelta(a, b Pose) Pose {
	return &distancePose{
		orientation: quat.Mul(b.Orientation().Quaternion(), quat.Conj(a.Orientation().Quaternion())),
		point:       b.Point().Sub(a.Point()),
	}
}

// PoseToProtobuf converts a pose to the pose format protobuf expects (which is as OrientationVectorDegrees).
func PoseToProtobuf(p Pose) *commonpb.Pose {
	final := &commonpb.Pose{}
	pt := p.Point()
	final.X = pt.X
	final.Y = pt.Y
	final.Z = pt.Z
	poseOV := p.Orientation().OrientationVectorDegrees()
	final.Theta = poseOV.Theta
	final.OX = poseOV.OX
	final.OY = poseOV.OY
	final.OZ = poseOV.OZ
	return final
}

// PoseInverse will return the inverse of a pose. So if a given pose p is the pose of A relative to B, PoseInverse(p) will give
// the pose of B relative to A.
func PoseInverse(p Pose) Pose {
	return newDualQuaternionFromPose(p).Invert()
}

// Interpolate will return a new Pose that has been interpolated the set amount between two poses.
// Note that position and orientation are interpolated separately, then the two are combined.
// Note that slerp(q1, q2) != slerp(q2, q1)
// p1 and p2 are the two poses to interpolate between, by is a float representing the amount to interpolate between them.
// by == 0 will return p1, by == 1 will return p2, and by == 0.5 will return the pose halfway between them.
func Interpolate(p1, p2 Pose, by float64) Pose {
	intQ := newDualQuaternion()
	intQ.Real = slerp(p1.Orientation().Quaternion(), p2.Orientation().Quaternion(), by)

	intQ.SetTranslation(r3.Vector{
		(p1.Point().X + (p2.Point().X-p1.Point().X)*by),
		(p1.Point().Y + (p2.Point().Y-p1.Point().Y)*by),
		(p1.Point().Z + (p2.Point().Z-p1.Point().Z)*by),
	})
	return intQ
}

// PoseAlmostEqual will return a bool describing whether 2 poses are approximately the same.
func PoseAlmostEqual(a, b Pose) bool {
	return PoseAlmostCoincident(a, b) && OrientationAlmostEqual(a.Orientation(), b.Orientation())
}

// PoseAlmostEqualEps will return a bool describing whether 2 poses are approximately the same.
func PoseAlmostEqualEps(a, b Pose, epsilon float64) bool {
	return PoseAlmostCoincidentEps(a, b, epsilon) && OrientationAlmostEqual(a.Orientation(), b.Orientation())
}

// PoseAlmostCoincident will return a bool describing whether 2 poses approximately are at the same 3D coordinate location.
// This uses the same epsilon as the default value for the Viam IK solver.
func PoseAlmostCoincident(a, b Pose) bool {
	return PoseAlmostCoincidentEps(a, b, defaultDistanceEpsilon)
}

// PoseAlmostCoincidentEps will return a bool describing whether 2 poses approximately are at the same 3D coordinate location.
// This uses a passed in epsilon value.
func PoseAlmostCoincidentEps(a, b Pose, epsilon float64) bool {
	return R3VectorAlmostEqual(a.Point(), b.Point(), epsilon)
}

// distancePose holds an already computed pose and orientation. It is not efficient to do spatial math on a
// distancePose, use a dualQuaternion instead.
// A distancePose is useful when you need to return e.g. a computed point within a pose without converting back to a DQ.
type distancePose struct {
	point       r3.Vector
	orientation quat.Number
}

func (d *distancePose) Point() r3.Vector {
	return d.point
}

func (d *distancePose) Orientation() Orientation {
	return (*Quaternion)(&d.orientation)
}

// ResetPoseDQTranslation takes a Pose that must be a dualQuaternion and reset's it's translation.
func ResetPoseDQTranslation(p Pose, v r3.Vector) {
	q, ok := p.(*dualQuaternion)
	if !ok {
		panic("ResetPoseDQTranslation has to be passed a dual quaternion")
	}
	q.SetTranslation(v)
}
