package arm_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	componentpb "go.viam.com/api/component/arm/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	gotestutils "go.viam.com/utils/testutils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var (
		capArmPos      spatialmath.Pose
		capArmJointPos *componentpb.JointPositions
		extraOptions   map[string]interface{}
	)

	pos1 := spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})
	jointPos1 := &componentpb.JointPositions{Values: []float64{1.0, 2.0, 3.0}}
	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		extraOptions = extra
		return pos1, nil
	}
	injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*componentpb.JointPositions, error) {
		extraOptions = extra
		return jointPos1, nil
	}
	injectArm.MoveToPositionFunc = func(
		ctx context.Context,
		ap spatialmath.Pose,
		worldState *referenceframe.WorldState,
		extra map[string]interface{},
	) error {
		capArmPos = ap
		extraOptions = extra
		return nil
	}

	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *componentpb.JointPositions, extra map[string]interface{}) error {
		capArmJointPos = jp
		extraOptions = extra
		return nil
	}
	injectArm.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extraOptions = extra
		return arm.ErrStopUnimplemented
	}

	pos2 := spatialmath.NewPoseFromPoint(r3.Vector{X: 4, Y: 5, Z: 6})
	jointPos2 := &componentpb.JointPositions{Values: []float64{4.0, 5.0, 6.0}}
	injectArm2 := &inject.Arm{}
	injectArm2.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pos2, nil
	}
	injectArm2.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*componentpb.JointPositions, error) {
		return jointPos2, nil
	}
	injectArm2.MoveToPositionFunc = func(
		ctx context.Context,
		ap spatialmath.Pose,
		worldState *referenceframe.WorldState,
		extra map[string]interface{},
	) error {
		capArmPos = ap
		return nil
	}

	injectArm2.MoveToJointPositionsFunc = func(ctx context.Context, jp *componentpb.JointPositions, extra map[string]interface{}) error {
		capArmJointPos = jp
		return nil
	}
	injectArm2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}

	armSvc, err := subtype.New(map[resource.Name]interface{}{arm.Named(testArmName): injectArm, arm.Named(testArmName2): injectArm2})
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(arm.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, armSvc)

	generic.RegisterService(rpcServer, armSvc)
	injectArm.DoFunc = generic.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	t.Run("arm client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		arm1Client := arm.NewClientFromConn(context.Background(), conn, testArmName, logger)

		// DoCommand
		resp, err := arm1Client.DoCommand(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		pos, err := arm1Client.EndPosition(context.Background(), map[string]interface{}{"foo": "EndPosition"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqual(pos, pos1), test.ShouldBeTrue)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "EndPosition"})

		jointPos, err := arm1Client.JointPositions(context.Background(), map[string]interface{}{"foo": "JointPositions"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, jointPos.String(), test.ShouldResemble, jointPos1.String())
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "JointPositions"})

		err = arm1Client.MoveToPosition(context.Background(), pos2, &referenceframe.WorldState{}, map[string]interface{}{"foo": "MoveToPosition"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqual(capArmPos, pos2), test.ShouldBeTrue)

		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "MoveToPosition"})

		err = arm1Client.MoveToJointPositions(context.Background(), jointPos2, map[string]interface{}{"foo": "MoveToJointPositions"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos2.String())
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "MoveToJointPositions"})

		err = arm1Client.Stop(context.Background(), map[string]interface{}{"foo": "Stop"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, arm.ErrStopUnimplemented.Error())
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "Stop"})

		test.That(t, utils.TryClose(context.Background(), arm1Client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("arm client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testArmName2, logger)
		arm2Client, ok := client.(arm.Arm)
		test.That(t, ok, test.ShouldBeTrue)

		pos, err := arm2Client.EndPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqual(pos, pos2), test.ShouldBeTrue)

		err = arm2Client.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectArm := &inject.Arm{}

	armSvc, err := subtype.New(map[resource.Name]interface{}{arm.Named(testArmName): injectArm})
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterArmServiceServer(gServer, arm.NewServer(armSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := arm.NewClientFromConn(ctx, conn1, testArmName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := arm.NewClientFromConn(ctx, conn2, testArmName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

func TestClientModel(t *testing.T) {
	logger := golog.NewTestLogger(t)

	// create inject arm
	var injectArmPosition *componentpb.JointPositions
	injectArm := &inject.Arm{}
	injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*componentpb.JointPositions, error) {
		return injectArmPosition, nil
	}
	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *componentpb.JointPositions, extra map[string]interface{}) error {
		injectArmPosition = jp
		return nil
	}

	// create basic Model for arm
	json := `{"name": "foo","joints": [{"id": "bar","type": "revolute","parent": "world","axis": {"x": 1},"max": 360,"min": -360}]}`

	model, err := referenceframe.UnmarshalModelJSON([]byte(json), "")
	test.That(t, err, test.ShouldBeNil)

	// create inject Robot
	injectRobot := &inject.Robot{}
	injectRobot.FrameSystemConfigFunc = func(
		ctx context.Context,
		additionalTransforms []*referenceframe.LinkInFrame,
	) (framesystemparts.Parts, error) {
		return framesystemparts.Parts{&referenceframe.FrameSystemPart{
			FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, nil, testArmName, nil),
			ModelFrame:  model,
		}}, nil
	}

	// register services, setup connection
	var listener net.Listener = gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	robotpb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	armSvc, err := subtype.New(map[resource.Name]interface{}{arm.Named(testArmName): injectArm, arm.Named(testArmName2): injectArm})
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterArmServiceServer(gServer, arm.NewServer(armSvc))
	go gServer.Serve(listener)
	defer gServer.Stop()
	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	// test client
	armClient := arm.NewClientFromConn(context.Background(), conn, testArmName, logger)
	defer test.That(t, utils.TryClose(context.Background(), armClient), test.ShouldBeNil)

	modelResponse := armClient.ModelFrame()
	test.That(t, modelResponse, test.ShouldNotBeNil)
	expected := []referenceframe.Input{{Value: 90}}
	err = armClient.GoToInputs(context.Background(), expected)
	test.That(t, err, test.ShouldBeNil)
	actual, err := armClient.CurrentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, expected[0].Value, test.ShouldAlmostEqual, actual[0].Value)
}
