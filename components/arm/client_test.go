package arm_test

import (
	"context"
	"net"
	"testing"

	"github.com/golang/geo/r3"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var (
		capArmPos      spatialmath.Pose
		capArmJointPos []referenceframe.Input
		moveOptions    arm.MoveOptions
		extraOptions   map[string]interface{}
	)

	pos1 := spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})
	jointPos1 := []referenceframe.Input{{1.}, {2.}, {3.}}
	expectedGeometries := []spatialmath.Geometry{spatialmath.NewPoint(r3.Vector{1, 2, 3}, "")}
	expectedMoveOptions := arm.MoveOptions{MaxVel: 1, MaxAcc: 2}
	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		extraOptions = extra
		return pos1, nil
	}
	injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
		extraOptions = extra
		return jointPos1, nil
	}
	injectArm.MoveToPositionFunc = func(ctx context.Context, ap spatialmath.Pose, extra map[string]interface{}) error {
		capArmPos = ap
		extraOptions = extra
		return nil
	}
	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp []referenceframe.Input, extra map[string]interface{}) error {
		capArmJointPos = jp
		extraOptions = extra
		return nil
	}
	injectArm.MoveThroughJointPositionsFunc = func(
		ctx context.Context,
		positions [][]referenceframe.Input,
		options *arm.MoveOptions,
		extra map[string]interface{},
	) error {
		capArmJointPos = positions[len(positions)-1]
		moveOptions = *options
		extraOptions = extra
		return nil
	}
	injectArm.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extraOptions = extra
		return errStopUnimplemented
	}
	injectArm.ModelFrameFunc = func() referenceframe.Model {
		return nil
	}
	injectArm.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
		return expectedGeometries, nil
	}

	pos2 := spatialmath.NewPoseFromPoint(r3.Vector{X: 4, Y: 5, Z: 6})
	jointPos2 := []referenceframe.Input{{4.}, {5.}, {6.}}
	injectArm2 := &inject.Arm{}
	injectArm2.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pos2, nil
	}
	injectArm2.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
		return jointPos2, nil
	}
	injectArm2.MoveToPositionFunc = func(ctx context.Context, ap spatialmath.Pose, extra map[string]interface{}) error {
		capArmPos = ap
		return nil
	}
	injectArm2.MoveToJointPositionsFunc = func(ctx context.Context, jp []referenceframe.Input, extra map[string]interface{}) error {
		capArmJointPos = jp
		return nil
	}
	injectArm2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}
	injectArm2.ModelFrameFunc = func() referenceframe.Model {
		return nil
	}

	armSvc, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{
		arm.Named(testArmName):  injectArm,
		arm.Named(testArmName2): injectArm2,
	})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[arm.Arm](arm.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, armSvc), test.ShouldBeNil)

	injectRobot := &inject.Robot{}
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}
	test.That(t, rpcServer.RegisterServiceServer(
		context.Background(),
		&robotpb.RobotService_ServiceDesc,
		server.New(injectRobot),
	), test.ShouldBeNil)

	injectArm.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	// working
	t.Run("arm client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		arm1Client, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(testArmName), logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		resp, err := arm1Client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		pos, err := arm1Client.EndPosition(context.Background(), map[string]interface{}{"foo": "EndPosition"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqual(pos, pos1), test.ShouldBeTrue)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "EndPosition"})

		jointPos, err := arm1Client.JointPositions(context.Background(), map[string]interface{}{"foo": "JointPositions"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, jointPos, test.ShouldResemble, jointPos1)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "JointPositions"})

		err = arm1Client.MoveToPosition(context.Background(), pos2, map[string]interface{}{"foo": "MoveToPosition"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqual(capArmPos, pos2), test.ShouldBeTrue)

		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "MoveToPosition"})

		err = arm1Client.MoveToJointPositions(context.Background(), jointPos2, map[string]interface{}{"foo": "MoveToJointPositions"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos, test.ShouldResemble, jointPos2)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "MoveToJointPositions"})

 		err = arm1Client.MoveThroughJointPositions(
			context.Background(),
			[][]referenceframe.Input{jointPos2, jointPos1},
			&expectedMoveOptions,
			map[string]interface{}{"foo": "MoveThroughJointPositions"},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos, test.ShouldResemble, jointPos1)
		test.That(t, moveOptions, test.ShouldResemble, expectedMoveOptions)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "MoveThroughJointPositions"})

		err = arm1Client.Stop(context.Background(), map[string]interface{}{"foo": "Stop"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStopUnimplemented.Error())
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "Stop"})

		geometries, err := arm1Client.Geometries(context.Background(), map[string]interface{}{"foo": "Geometries"})
		test.That(t, err, test.ShouldBeNil)
		for i, geometry := range geometries {
			test.That(t, spatialmath.GeometriesAlmostEqual(expectedGeometries[i], geometry), test.ShouldBeTrue)
		}

		test.That(t, arm1Client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("arm client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", arm.Named(testArmName2), logger)
		test.That(t, err, test.ShouldBeNil)

		pos, err := client2.EndPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqual(pos, pos2), test.ShouldBeTrue)

		err = client2.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
