package referenceframe

import (
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

// Pose is any struct that represents a 6dof pose and can express that pose as a dual quaternion, and what frame it is in.
// The pose is the translation and orientation of some object relative to the origin of some Frame.
// FROM Frame -> TO Object ... not the other way around!
type Pose interface {
	DualQuat() *spatial.DualQuaternion
}

// Fulfills the Pose interface by directly using a dual quaternion.
type dualQuatPose struct {
	*spatial.DualQuaternion
}

// DualQuat returns the DualQuaternion useful for transforming between frames.
func (dqp *dualQuatPose) DualQuat() *spatial.DualQuaternion {
	return dqp.DualQuaternion
}

// NewEmptyPose returns a pose with no Frame and an identity unit quaternion
func NewEmptyPose() Pose {
	return &dualQuatPose{spatial.NewDualQuaternion()}
}

// NewPoseFromPoint takes in a cartesian (x,y,z) point and promotes it to a dual quaternion.
// It will have the exact orientation as the Frame it is in.
func NewPoseFromPoint(point r3.Vector) Pose {
	dq := &spatial.DualQuaternion{dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Number{Imag: point.X, Jmag: point.Y, Kmag: point.Z},
	}}
	return &dualQuatPose{dq}
}
