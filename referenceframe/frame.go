// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/spatialmath"
)

// OffsetBy takes two offsets and computes the final position.
func OffsetBy(a, b *pb.ArmPosition) *pb.ArmPosition {
	q1 := spatialmath.NewDualQuaternionFromArmPos(a)
	q2 := spatialmath.NewDualQuaternionFromArmPos(b)
	q3 := &spatialmath.DualQuaternion{q1.Transformation(q2.Quat)}

	return q3.ToArmPos()
}

// Frame represents a single reference frame, e.g. an arm, a joint, etc.
type Frame interface {
	Parent() string // TODO: make this not a string
	Transform([]Input) *spatialmath.DualQuaternion
	Dof() int
}

// Input wraps the input to a mutable frame, e.g. a joint angle or a gantry position.
// TODO: Determine what more this needs, or eschew in favor of raw float64s if nothing needed.
type Input struct {
	Value float64
}
