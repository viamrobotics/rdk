package arm_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	componentpb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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
		capArmPos      *commonpb.Pose
		capArmJointPos *componentpb.JointPositions
	)

	pos1 := &commonpb.Pose{X: 1, Y: 2, Z: 3}
	jointPos1 := &componentpb.JointPositions{Values: []float64{1.0, 2.0, 3.0}}
	injectArm := &inject.Arm{}
	injectArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos1, nil
	}
	injectArm.GetJointPositionsFunc = func(ctx context.Context) (*componentpb.JointPositions, error) {
		return jointPos1, nil
	}
	injectArm.MoveToPositionFunc = func(ctx context.Context, ap *commonpb.Pose, worldState *commonpb.WorldState) error {
		capArmPos = ap
		return nil
	}

	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *componentpb.JointPositions) error {
		capArmJointPos = jp
		return nil
	}
	injectArm.StopFunc = func(ctx context.Context) error {
		return arm.ErrStopUnimplemented
	}

	pos2 := &commonpb.Pose{X: 4, Y: 5, Z: 6}
	jointPos2 := &componentpb.JointPositions{Values: []float64{4.0, 5.0, 6.0}}
	injectArm2 := &inject.Arm{}
	injectArm2.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos2, nil
	}
	injectArm2.GetJointPositionsFunc = func(ctx context.Context) (*componentpb.JointPositions, error) {
		return jointPos2, nil
	}
	injectArm2.MoveToPositionFunc = func(ctx context.Context, ap *commonpb.Pose, worldState *commonpb.WorldState) error {
		capArmPos = ap
		return nil
	}

	injectArm2.MoveToJointPositionsFunc = func(ctx context.Context, jp *componentpb.JointPositions) error {
		capArmJointPos = jp
		return nil
	}
	injectArm2.StopFunc = func(ctx context.Context) error {
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

		// Do
		resp, err := arm1Client.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		pos, err := arm1Client.GetEndPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos.String(), test.ShouldResemble, pos1.String())

		jointPos, err := arm1Client.GetJointPositions(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, jointPos.String(), test.ShouldResemble, jointPos1.String())

		err = arm1Client.MoveToPosition(context.Background(), pos2, &commonpb.WorldState{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmPos.String(), test.ShouldResemble, pos2.String())

		err = arm1Client.MoveToJointPositions(context.Background(), jointPos2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos2.String())

		err = arm1Client.Stop(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, arm.ErrStopUnimplemented.Error())

		test.That(t, utils.TryClose(context.Background(), arm1Client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("arm client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testArmName2, logger)
		arm2Client, ok := client.(arm.Arm)
		test.That(t, ok, test.ShouldBeTrue)

		pos, err := arm2Client.GetEndPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos.String(), test.ShouldResemble, pos2.String())

		err = arm2Client.Stop(context.Background())
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
