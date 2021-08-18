package referenceframe

import (
	pb "go.viam.com/core/proto/api/v1"
	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
)

// OffsetBy takes two offsets and computes the final position.
func OffsetBy(a, b *pb.ArmPosition) *pb.ArmPosition {
	q1 := spatial.NewDualQuaternionFromArmPos(a)
	q2 := spatial.NewDualQuaternionFromArmPos(b)
	q3 := &spatial.DualQuaternion{q1.Transformation(q2.Number)}

	return q3.ToArmPos()
}

// Pose represents a 6dof pose, position and orientation. For convenience, everything is returned as a dual quaternion.
// translation is the translation operation (Δx,Δy,Δz), in this case [1, 0, 0 ,0][0, Δx/2, Δy/2, Δz/2] is returned
// orientation is often an SO(3) matrix, in this case [cos(th/2), nx*sin(th/2), ny*sin(th/2) , nz*sin(th/2)][0, 0, 0, 0] is returned
// Be aware that transform will take you FROM ObjectFrame -> TO ParentFrame ... not the other way around!
// To go the other direction, the Conjugate of a dual quaternion should be used
// You can also return the normal 3D point of the frame in space, as an (x,y,z) Vector.
// Resource for dual quaternion math: https://cs.gmu.edu/~jmlien/teaching/cs451/uploads/Main/dual-quaternion.pdf
type Pose interface {
	Point() r3.Vector
	Transform() *spatial.DualQuaternion // does a rotation first, then a translation
}

// basicPose stores a pose position as a 3D vector, and a pose orientation as a quaternion.
// orientation in quaternion form is expressed as [cos (theta/2), n * sin (theta/2)], n is a unit orientation vector.
// To turn the position and orientation into a transformation, can express that transform as a dual quaternion.
// Transform() has the following dual-quaternion form : real = [orientation], dual = [0.5*orientation*[0, position]]
// It is equivalent to doing a rotation first, and then a translation
type basicPose struct {
	*spatial.DualQuaternion
}

// Point returns the position of the object as a Vector
func (bp *basicPose) Point() r3.Vector {
	xyz := bp.Translation().Dual
	return r3.Vector{xyz.Imag, xyz.Jmag, xyz.Kmag}
}

// Transform returns the transform of the object as a dual quaternion, which is equivalent to a rotation, then translation.
func (bp *basicPose) Transform() *spatial.DualQuaternion {
	return bp.DualQuaternion
}

// NewEmptyPose returns a pose at (0,0,0) with same orientation as whatever frame it is placed in.
func NewEmptyPose() Pose {
	return &basicPose{spatial.NewDualQuaternion()}
}

// NewPoseFromAxisAngle takes in a positon, rotationAxis, and angle and returns a Pose.
// angle is input in degrees.
func NewPoseFromAxisAngle(point, rotationAxis r3.Vector, angle float64) Pose {
	emptyVec := r3.Vector{0, 0, 0}
	if rotationAxis == emptyVec || angle == 0 {
		return &basicPose{spatial.NewDualQuaternion()}
	}
	aa := spatial.R4AA{Theta: angle, RX: rotationAxis.X, RY: rotationAxis.Y, RZ: rotationAxis.Z}
	
	quat := spatial.NewDualQuaternion()
	quat.Real = aa.ToQuat()
	quat.SetTranslation(point.X, point.Y, point.Z)
	return &basicPose{quat}
}

// NewPoseFromPoint takes in a cartesian (x,y,z) and stores it as a vector.
// It will have the same orientation as the frame it is in.
func NewPoseFromPoint(point r3.Vector) Pose {
	quat := spatial.NewDualQuaternion()
	quat.SetTranslation(point.X, point.Y, point.Z)
	return &basicPose{quat}
}

// NewPoseFromTransform splits a dual quaternion into a position vector and a orientation quaternion.
func NewPoseFromTransform(dq *spatial.DualQuaternion) Pose {
	return &basicPose{dq}
}

// TransformPoint applies a rotation and translation to a 3D point using a dual quaternion
func TransformPoint(transform *spatial.DualQuaternion, p r3.Vector) r3.Vector {
	pointQuat := spatial.NewDualQuaternion()
	pointQuat.SetTranslation(p.X, p.Y, p.Z)

	finalPose := basicPose{spatial.Compose(transform, pointQuat)}
	return finalPose.Point()
}
