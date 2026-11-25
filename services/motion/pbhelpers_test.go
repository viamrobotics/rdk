package motion

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/motionplan"
)

func TestMoveReqToProtoRejectsFrozenFrameConstraint(t *testing.T) {
	req := MoveReq{ComponentName: "arm", Constraints: &motionplan.Constraints{
		FrozenFrameConstraint: []motionplan.FrozenFrameConstraint{{Frame: "gantry"}},
	}}
	_, err := req.ToProto("builtin")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "frozen_frame_constraints")

	req.Constraints = &motionplan.Constraints{}
	_, err = req.ToProto("builtin")
	test.That(t, err, test.ShouldBeNil)
}
