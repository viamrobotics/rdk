package motion

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/motionplan"
)

func TestMoveReqToProtoRejectsFrozenFrames(t *testing.T) {
	req := MoveReq{ComponentName: "arm", Constraints: &motionplan.Constraints{FrozenFrames: []string{"gantry"}}}
	_, err := req.ToProto("builtin")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "frozen_frames")

	req.Constraints = &motionplan.Constraints{}
	_, err = req.ToProto("builtin")
	test.That(t, err, test.ShouldBeNil)
}
