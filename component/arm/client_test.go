package arm_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/utils"
	rpcclient "go.viam.com/utils/rpc/client"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/component/arm"
	commonpb "go.viam.com/core/proto/api/common/v1"
	componentpb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()

	var (
		capArmPos           *commonpb.Pose
		capArmJointPos      *componentpb.ArmJointPositions
		capArmJoint         int
		capArmJointAngleDeg float64
	)

	arm1 := "arm1"
	pos1 := &commonpb.Pose{X: 1, Y: 2, Z: 3}
	jointPos1 := &componentpb.ArmJointPositions{Degrees: []float64{1.0, 2.0, 3.0}}
	injectArm := &inject.Arm{}
	injectArm.CurrentPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos1, nil
	}
	injectArm.CurrentJointPositionsFunc = func(ctx context.Context) (*componentpb.ArmJointPositions, error) {
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

	injectArm.JointMoveDeltaFunc = func(ctx context.Context, joint int, amountDegs float64) error {
		capArmJoint = joint
		capArmJointAngleDeg = amountDegs
		return nil
	}

	arm2 := "arm2"
	pos2 := &commonpb.Pose{X: 4, Y: 5, Z: 6}
	jointPos2 := &componentpb.ArmJointPositions{Degrees: []float64{4.0, 5.0, 6.0}}
	injectArm2 := &inject.Arm{}
	injectArm2.CurrentPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos2, nil
	}
	injectArm2.CurrentJointPositionsFunc = func(ctx context.Context) (*componentpb.ArmJointPositions, error) {
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

	injectArm2.JointMoveDeltaFunc = func(ctx context.Context, joint int, amountDegs float64) error {
		capArmJoint = joint
		capArmJointAngleDeg = amountDegs
		return nil
	}

	armSvc, err := subtype.New((map[resource.Name]interface{}{arm.Named(arm1): injectArm, arm.Named(arm2): injectArm2}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterArmSubtypeServiceServer(gServer1, arm.NewServer(armSvc))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = arm.NewClient(cancelCtx, arm1, listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	arm1Client, err := arm.NewClient(context.Background(), arm1, listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("arm client 1", func(t *testing.T) {
		pos, err := arm1Client.CurrentPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos.String(), test.ShouldResemble, pos1.String())

		jointPos, err := arm1Client.CurrentJointPositions(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, jointPos.String(), test.ShouldResemble, jointPos1.String())

		err = arm1Client.MoveToPosition(context.Background(), pos2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmPos.String(), test.ShouldResemble, pos2.String())

		err = arm1Client.MoveToJointPositions(context.Background(), jointPos2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos2.String())

		err = arm1Client.JointMoveDelta(context.Background(), 10, 7.0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJoint, test.ShouldEqual, 10)
		test.That(t, capArmJointAngleDeg, test.ShouldEqual, 7.0)
	})

	t.Run("arm client 2", func(t *testing.T) {
		arm2Client, err := arm.NewFromClient(context.Background(), arm1Client, arm2)
		test.That(t, err, test.ShouldBeNil)

		pos, err := arm2Client.CurrentPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos.String(), test.ShouldResemble, pos2.String())

		jointPos, err := arm2Client.CurrentJointPositions(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, jointPos.String(), test.ShouldResemble, jointPos2.String())

		err = arm2Client.MoveToPosition(context.Background(), pos1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmPos.String(), test.ShouldResemble, pos1.String())

		err = arm2Client.MoveToJointPositions(context.Background(), jointPos1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos1.String())

		err = arm2Client.JointMoveDelta(context.Background(), 15, 57.0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArmJoint, test.ShouldEqual, 15)
		test.That(t, capArmJointAngleDeg, test.ShouldEqual, 57.0)
		test.That(t, utils.TryClose(arm2Client), test.ShouldBeNil)
	})

	t.Run("arm client 3", func(t *testing.T) {
		armSubtypeClient, err := arm.NewSubtypeClient(
			context.Background(), listener1.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger,
		)
		test.That(t, err, test.ShouldBeNil)
		arm1Client2, err := arm.NewClientFromSubtypeClient(armSubtypeClient, arm1)
		test.That(t, err, test.ShouldBeNil)
		pos, err := arm1Client2.CurrentPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos.String(), test.ShouldResemble, pos1.String())
		test.That(t, armSubtypeClient.Close(), test.ShouldBeNil)
	})
	test.That(t, utils.TryClose(arm1Client), test.ShouldBeNil)
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
	componentpb.RegisterArmSubtypeServiceServer(gServer, arm.NewServer(armSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &trackingDialer{Dialer: dialer.NewCachedDialer()}
	ctx := dialer.ContextWithDialer(context.Background(), td)
	client1, err := arm.NewClient(ctx, arm1, listener.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	client2, err := arm.NewClient(ctx, arm1, listener.Addr().String(), rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.dialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(client2)
	test.That(t, err, test.ShouldBeNil)
}

type trackingDialer struct {
	dialer.Dialer
	dialCalled int
}

func (td *trackingDialer) DialDirect(ctx context.Context, target string, opts ...grpc.DialOption) (dialer.ClientConn, error) {
	td.dialCalled++
	return td.Dialer.DialDirect(ctx, target, opts...)
}

func (td *trackingDialer) DialFunc(proto string, target string, f func() (dialer.ClientConn, error)) (dialer.ClientConn, error) {
	td.dialCalled++
	return td.Dialer.DialFunc(proto, target, f)
}
