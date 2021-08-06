package referenceframe

import (
	"fmt"
	"math"

	pb "go.viam.com/core/proto/api/v1"
	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

// OffsetBy takes two offsets and computes the final position.
func OffsetBy(a, b *pb.ArmPosition) *pb.ArmPosition {
	q1 := spatial.NewDualQuaternionFromArmPos(a)
	q2 := spatial.NewDualQuaternionFromArmPos(b)
	q3 := &spatial.DualQuaternion{q1.Transformation(q2.Quat)}

	return q3.ToArmPos()
}

// Compose takes two dual quaternions and multiplies them together, and then normalizes the transform.
// DualQuaternions apply their operation TO THE RIGHT. example: if you have an operation A and operation B on p
// BAp means (B(Ap)). First A is applied, then B. QUATERNIONS DO NOT COMMUTE! BAp =/= ABp!
func Compose(b, a dualquat.Number) dualquat.Number {
	result := dualquat.Mul(b, a)
	magnitude := quat.Mul(result.Real, quat.Conj(result.Real)).Real
	if magnitude < 1e-8 {
		panic("magnitude of dual quaternion is zero, cannot normalize")
	}
	result = dualquat.Scale(1./magnitude, result)
	return result
}

// Pose represents a 6dof pose, position and orientation. For convenience, everything is returned as a dual quaternion.
// translation is the translation operation (Δx,Δy,Δz), in this case [1, 0, 0 ,0][0, Δx/2, Δy/2, Δz/2] is returned
// orientation is often an SO(3) matrix, in this case [cos(th/2), nx*sin(th/2), ny*sin(th/2) , nz*sin(th/2)][0, 0, 0, 0] is returned
// Be aware that transform will take you FROM ObjectFrame -> TO ParentFrame ... not the other way around!
// You can also return the normal 3D point of the frame in space, as an (x,y,z) Vector.
type Pose interface {
	Point() r3.Vector
	Translation() dualquat.Number
	Rotation() dualquat.Number
	Transform() dualquat.Number // does a rotation first, then a translation
}

// basicPose stores position as a 3D vector, and orientation as a quaternion.
// orientation in quaternion form is expressed as [cos (theta/2), n * sin (theta/2)], n is a unit orientation vector.
// To turn the position and orientation into a transformation, can express that transform as a dual quaternion.
// Transform() has the following dual-quaternion form : real = [orientation], dual = [0.5*orientation*[0, position]]
// It is equivalent to doing a rotation first, and then a translation
type basicPose struct {
	position    r3.Vector
	orientation quat.Number
}

// Point returns the position of the object as a Vector
func (bp *basicPose) Point() r3.Vector {
	return bp.position
}

// Translation returns the pure translation of the object as a dual quaternion
func (bp *basicPose) Translation() dualquat.Number {
	translation := dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{Imag: bp.position.X / 2., Jmag: bp.position.Y / 2., Kmag: bp.position.Z / 2.},
	}
	return translation
}

// Orientation returns the pure rotation of the object as a dual quaternion
func (bp *basicPose) Rotation() dualquat.Number {
	orientation := dualquat.Number{
		Real: bp.orientation,
		Dual: quat.Number{},
	}
	return orientation
}

// Transform returns the transform of the object as a dual quaternion, which is equivalent to a rotation, then translation.
func (bp *basicPose) Transform() dualquat.Number {
	realpart := bp.orientation
	position := quat.Number{Imag: bp.position.X, Jmag: bp.position.Y, Kmag: bp.position.Z}
	dualpart := quat.Scale(0.5, quat.Mul(position, bp.orientation))

	transform := dualquat.Number{
		Real: realpart,
		Dual: dualpart,
	}
	return transform
}

// NewEmptyPose returns a pose at (0,0,0) with same orientation as whatever frame it is placed in.
func NewEmptyPose() Pose {
	return &basicPose{r3.Vector{}, quat.Number{Real: 1}}
}

// NewPose creates a pose directly from a position vector and an orientation quaternion.
func NewPose(point r3.Vector, orientation quat.Number) Pose {
	return &basicPose{point, orientation}
}

// NewPoseFromAxisAngle takes in a positon, rotationAxis, and angle and returns a Pose.
// angle is input in degrees.
func NewPoseFromAxisAngle(point, rotationAxis r3.Vector, angle float64) Pose {
	// normalize rotation axis and put in quaternion form
	axis := rotationAxis.Normalize()
	rot := angle * (math.Pi / 180.) / 2.
	rotation := quat.Number{math.Cos(rot), axis.X * math.Sin(rot), axis.Y * math.Sin(rot), axis.Z * math.Sin(rot)}
	return &basicPose{point, rotation}
}

// NewPoseFromPoint takes in a cartesian (x,y,z) and stores it as a vector.
// It will have the same orientation as the frame it is in.
func NewPoseFromPoint(point r3.Vector) Pose {
	return &basicPose{point, quat.Number{Real: 1}}
}

// NewPoseFromPointDualQuat takes in a point represented as a dual quaternion and stores it as a vector.
// It will have the same orientation as the Frame it is in.
func NewPoseFromPointDualQuat(point dualquat.Number) (Pose, error) {
	emptyQuat := quat.Number{1, 0, 0, 0}
	if !almostEqual(point.Real, emptyQuat) || point.Dual.Real != 0. {
		return nil, fmt.Errorf("input dual quaternion %v is not a point", point)
	}
	return &basicPose{r3.Vector{point.Dual.Imag, point.Dual.Jmag, point.Dual.Kmag}, quat.Number{Real: 1}}, nil
}

// NewPoseFromTransform splits a dual quaternion into a position vector and a orientation quaternion.
func NewPoseFromTransform(dq dualquat.Number) Pose {
	rotation := dq.Real
	t := quat.Scale(2., quat.Mul(dq.Dual, quat.Conj(dq.Real)))
	position := r3.Vector{t.Imag, t.Jmag, t.Kmag}
	return &basicPose{position, rotation}
}

// TransformPoint applies a rotation and translation to a 3D point using a dual quaternion
func TransformPoint(transform dualquat.Number, p r3.Vector) (r3.Vector, error) {
	transformed := dualquat.Mul(dualquat.Mul(transform, pointDualQuat(p)), dualquat.Conj(transform))
	result, err := NewPoseFromPointDualQuat(transformed)
	if err != nil {
		return r3.Vector{}, err
	}
	return result.Point(), nil
}

// pointDualQuat puts the position of the object in a dual quaternion form for convenience (DO NOT USE FOR TRANSLATIONS)
func pointDualQuat(point r3.Vector) dualquat.Number {
	position := dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{Imag: point.X, Jmag: point.Y, Kmag: point.Z},
	}
	return position
}

// equality test for all the float components of a quaternion
func almostEqual(a, b quat.Number) bool {
	eps := 1e-6
	if math.Abs(a.Real-b.Real) > eps {
		return false
	}
	if math.Abs(a.Imag-b.Imag) > eps {
		return false
	}
	if math.Abs(a.Jmag-b.Jmag) > eps {
		return false
	}
	if math.Abs(a.Kmag-b.Kmag) > eps {
		return false
	}
	return true
}
