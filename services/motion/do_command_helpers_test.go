package motion_test

import (
	"context"
	"net"
	"testing"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/encoding/protojson"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	injectmotion "go.viam.com/rdk/testutils/inject/motion"
)

func TestClientDoPlan(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectMS := injectmotion.NewMotionService("test")
	coll, err := resource.NewAPIResourceCollection(motion.API, map[resource.Name]motion.Service{testMotionServiceName: injectMS})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[motion.Service](motion.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(ctx, rpcServer, coll, logger), test.ShouldBeNil)
	go rpcServer.Serve(listener)
	defer func() { test.That(t, rpcServer.Stop(), test.ShouldBeNil) }()

	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, conn.Close(), test.ShouldBeNil) }()
	client, err := motion.NewClientFromConn(ctx, conn, "", testMotionServiceName, logger)
	test.That(t, err, test.ShouldBeNil)

	req := motion.MoveReq{
		ComponentName: "arm",
		Destination:   referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})),
		Extra:         map[string]interface{}{"timeout": 5.0},
	}
	traj := motionplan.Trajectory{
		{"arm": {0, 1, 2}},
		{"arm": {0.5, 1.5, 2.5}, "gripper": {}},
	}

	var gotCmd map[string]interface{}
	respondWith := func(resp map[string]interface{}) {
		injectMS.DoCommandFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			gotCmd = cmd
			return resp, nil
		}
	}

	t.Run("MoveReq is sent as protojson and the response is typed", func(t *testing.T) {
		respondWith(map[string]interface{}{motion.DoPlan: traj})
		resp, err := client.DoCommand(ctx, map[string]interface{}{motion.DoPlan: req})
		test.That(t, err, test.ShouldBeNil)

		s, ok := gotCmd[motion.DoPlan].(string)
		test.That(t, ok, test.ShouldBeTrue)
		var reqPB pb.MoveRequest
		test.That(t, protojson.Unmarshal([]byte(s), &reqPB), test.ShouldBeNil)
		test.That(t, reqPB.Name, test.ShouldEqual, testMotionServiceName.Name)
		got, err := motion.MoveReqFromProto(&reqPB)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got.ComponentName, test.ShouldEqual, req.ComponentName)
		test.That(t, got.Extra["timeout"], test.ShouldEqual, 5.0)

		test.That(t, resp[motion.DoPlan], test.ShouldResemble, traj)
		test.That(t, resp, test.ShouldNotContainKey, motion.DoPlanPartialWaypoint)
	})

	t.Run("pointer MoveReq and partial plan", func(t *testing.T) {
		respondWith(map[string]interface{}{motion.DoPlan: traj, motion.DoPlanPartialWaypoint: 3})
		resp, err := client.DoCommand(ctx, map[string]interface{}{motion.DoPlan: &req})
		test.That(t, err, test.ShouldBeNil)
		_, isString := gotCmd[motion.DoPlan].(string)
		test.That(t, isString, test.ShouldBeTrue)
		test.That(t, resp[motion.DoPlan], test.ShouldResemble, traj)
		test.That(t, resp[motion.DoPlanPartialWaypoint], test.ShouldEqual, 3)
	})

	t.Run("string request and its response pass through unchanged", func(t *testing.T) {
		respondWith(map[string]interface{}{motion.DoPlan: traj, motion.DoPlanPartialWaypoint: 3})
		resp, err := client.DoCommand(ctx, map[string]interface{}{motion.DoPlan: "raw"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCmd[motion.DoPlan], test.ShouldEqual, "raw")
		_, isTyped := resp[motion.DoPlan].(motionplan.Trajectory)
		test.That(t, isTyped, test.ShouldBeFalse)
		_, isList := resp[motion.DoPlan].([]interface{})
		test.That(t, isList, test.ShouldBeTrue)
		test.That(t, resp[motion.DoPlanPartialWaypoint], test.ShouldEqual, 3.0)
	})

	t.Run("undecodable response values are left as is", func(t *testing.T) {
		respondWith(map[string]interface{}{motion.DoPlan: "not a trajectory", motion.DoPlanPartialWaypoint: "x"})
		resp, err := client.DoCommand(ctx, map[string]interface{}{motion.DoPlan: req})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp[motion.DoPlan], test.ShouldEqual, "not a trajectory")
		test.That(t, resp[motion.DoPlanPartialWaypoint], test.ShouldEqual, "x")
	})

	t.Run("other commands are untouched", func(t *testing.T) {
		respondWith(map[string]interface{}{motion.DoPlan: []interface{}{}})
		resp, err := client.DoCommand(ctx, map[string]interface{}{"other": 1.0})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotCmd, test.ShouldResemble, map[string]interface{}{"other": 1.0})
		test.That(t, resp[motion.DoPlan], test.ShouldResemble, []interface{}{})
	})
}
