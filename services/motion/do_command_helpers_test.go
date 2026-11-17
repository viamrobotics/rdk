package motion_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	injectmotion "go.viam.com/rdk/testutils/inject/motion"
)

func TestPlanHelper(t *testing.T) {
	ctx := context.Background()
	req := motion.MoveReq{
		ComponentName: "arm",
		Destination:   referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})),
		Extra:         map[string]interface{}{"timeout": 5.0},
	}
	traj := motionplan.Trajectory{
		{"arm": {0, 1, 2}},
		{"arm": {0.5, 1.5, 2.5}, "gripper": {}},
	}

	overWire := func(t *testing.T, resp map[string]interface{}) map[string]interface{} {
		t.Helper()
		pbResp, err := protoutils.StructToStructPb(resp)
		test.That(t, err, test.ShouldBeNil)
		return pbResp.AsMap()
	}

	newSvc := func(resp map[string]interface{}) (*injectmotion.MotionService, *map[string]interface{}) {
		var gotCmd map[string]interface{}
		ms := injectmotion.NewMotionService("my motion")
		ms.DoCommandFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			gotCmd = cmd
			return resp, nil
		}
		return ms, &gotCmd
	}

	t.Run("encodes request as protojson MoveRequest", func(t *testing.T) {
		ms, gotCmd := newSvc(map[string]interface{}{motion.DoPlan: traj})
		_, err := motion.Plan(ctx, ms, req)
		test.That(t, err, test.ShouldBeNil)

		s, ok := (*gotCmd)[motion.DoPlan].(string)
		test.That(t, ok, test.ShouldBeTrue)
		var reqPB pb.MoveRequest
		test.That(t, protojson.Unmarshal([]byte(s), &reqPB), test.ShouldBeNil)
		test.That(t, reqPB.Name, test.ShouldEqual, "my motion")
		got, err := motion.MoveReqFromProto(&reqPB)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got.ComponentName, test.ShouldEqual, req.ComponentName)
		test.That(t, got.Extra["timeout"], test.ShouldEqual, 5.0)
	})

	t.Run("decodes in-process trajectory", func(t *testing.T) {
		ms, _ := newSvc(map[string]interface{}{motion.DoPlan: traj})
		result, err := motion.Plan(ctx, ms, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result.Trajectory, test.ShouldResemble, traj)
		test.That(t, result.PartialWaypoint, test.ShouldBeNil)
	})

	t.Run("decodes wire-shaped trajectory", func(t *testing.T) {
		resp := overWire(t, map[string]interface{}{motion.DoPlan: traj})
		_, isTyped := resp[motion.DoPlan].(motionplan.Trajectory)
		test.That(t, isTyped, test.ShouldBeFalse)

		ms, _ := newSvc(resp)
		result, err := motion.Plan(ctx, ms, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result.Trajectory), test.ShouldEqual, len(traj))
		test.That(t, result.Trajectory[0]["arm"], test.ShouldResemble, traj[0]["arm"])
		test.That(t, result.Trajectory[1]["arm"], test.ShouldResemble, traj[1]["arm"])
		test.That(t, result.PartialWaypoint, test.ShouldBeNil)
	})

	t.Run("decodes partial plan in-process and over the wire", func(t *testing.T) {
		raw := map[string]interface{}{motion.DoPlan: traj, motion.DoPlanPartialWaypoint: 3}
		for _, resp := range []map[string]interface{}{raw, overWire(t, raw)} {
			ms, _ := newSvc(resp)
			result, err := motion.Plan(ctx, ms, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, result.PartialWaypoint, test.ShouldNotBeNil)
			test.That(t, *result.PartialWaypoint, test.ShouldEqual, 3)
		}
	})

	t.Run("missing plan key errors", func(t *testing.T) {
		ms, _ := newSvc(map[string]interface{}{})
		_, err := motion.Plan(ctx, ms, req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, motion.DoPlan)
	})
}
