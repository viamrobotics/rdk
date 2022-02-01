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
	viamgrpc "go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()

	var (
		capArmPos      *commonpb.Pose
		capArmJointPos *componentpb.ArmJointPositions
	)

	arm1 := "arm1"
	pos1 := &commonpb.Pose{X: 1, Y: 2, Z: 3}
	jointPos1 := &componentpb.ArmJointPositions{Degrees: []float64{1.0, 2.0, 3.0}}
	injectArm := &inject.Arm{}
	injectArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos1, nil
	}
	injectArm.GetJointPositionsFunc = func(ctx context.Context) (*componentpb.ArmJointPositions, error) {
		return jointPos1, nil
	}
	injectArm.MoveToPositionFunc = func(ctx context.Context, ap *commonpb.Pose) error {
		capArmPos = ap
		return nil
	}

	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *componentpb.ArmJointPositions) error {
		capArmJointPos = jp
		return nil
	}

	arm2 := "arm2"
	pos2 := &commonpb.Pose{X: 4, Y: 5, Z: 6}
	jointPos2 := &componentpb.ArmJointPositions{Degrees: []float64{4.0, 5.0, 6.0}}
	injectArm2 := &inject.Arm{}
	injectArm2.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos2, nil
	}
	injectArm2.GetJointPositionsFunc = func(ctx context.Context) (*componentpb.ArmJointPositions, error) {
		return jointPos2, nil
	}
	injectArm2.MoveToPositionFunc = func(ctx context.Context, ap *commonpb.Pose) error {
		capArmPos = ap
		return nil
	}

	injectArm2.MoveToJointPositionsFunc = func(ctx context.Context, jp *componentpb.ArmJointPositions) error {
		capArmJointPos = jp
		return nil
	}

	armSvc, err := subtype.New((map[resource.Name]interface{}{arm.Named(arm1): injectArm, arm.Named(arm2): injectArm2}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterArmServiceServer(gServer1, arm.NewServer(armSvc))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = arm.NewClient(cancelCtx, arm1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	arm1Client, err := arm.NewClient(context.Background(), arm1, listener1.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)

	t.Run("arm client 1", func(t *testing.T) {
		pos, err := arm1Client.GetEndPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos.String(), test.ShouldResemble, pos1.String())

		jointPos, err := arm1Client.GetJointPositions(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, jointPos.String(), test.ShouldResemble, jointPos1.String())

		err = arm1Client.MoveToPosition(context.Background(), pos2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmPos.String(), test.ShouldResemble, pos2.String())

		err = arm1Client.MoveToJointPositions(context.Background(), jointPos2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos2.String())
	})

	t.Run("arm client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		arm1Client2 := arm.NewClientFromConn(context.Background(), conn, arm1, logger)
		test.That(t, err, test.ShouldBeNil)
		pos, err := arm1Client2.GetEndPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos.String(), test.ShouldResemble, pos1.String())
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, utils.TryClose(context.Background(), arm1Client), test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectArm := &inject.Arm{}
	arm1 := "arm1"

	armSvc, err := subtype.New((map[resource.Name]interface{}{arm.Named(arm1): injectArm}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterArmServiceServer(gServer, arm.NewServer(armSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := arm.NewClient(ctx, arm1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := arm.NewClient(ctx, arm1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
