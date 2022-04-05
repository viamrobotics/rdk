package motion_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gripper"
	viamgrpc "go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	servicepb "go.viam.com/rdk/proto/api/service/motion/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
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

	injectMS := &inject.MotionService{}
	omMap := map[resource.Name]interface{}{
		motion.Name: injectMS,
	}
	svc, err := subtype.New(omMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(motion.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)
	grabPose := referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())
	resourceName := gripper.Named("fake")
	test.That(t, err, test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = motion.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	t.Run("motion client 1", func(t *testing.T) {
		client, err := motion.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		success := true
		injectMS.MoveFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destination *referenceframe.PoseInFrame,
			worldState *commonpb.WorldState,
		) (bool, error) {
			return success, nil
		}
		injectMS.GetPoseFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destinationFrame string,
		) (*referenceframe.PoseInFrame, error) {
			return referenceframe.NewPoseInFrame(
				destinationFrame+componentName.Name, spatialmath.NewPoseFromPoint(r3.Vector{1, 2, 3})), nil
		}

		result, err := client.Move(context.Background(), resourceName, grabPose, &commonpb.WorldState{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldEqual, success)

		poseResult, err := client.GetPose(context.Background(), arm.Named("arm1"), "foo")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, poseResult.FrameName(), test.ShouldEqual, "fooarm1")
		test.That(t, poseResult.Pose().Point().X, test.ShouldEqual, 1)
		test.That(t, poseResult.Pose().Point().Y, test.ShouldEqual, 2)
		test.That(t, poseResult.Pose().Point().Z, test.ShouldEqual, 3)

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	// broken
	t.Run("motion client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		client2, ok := client.(motion.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("fake move error")
		injectMS.MoveFunc = func(
			ctx context.Context,
			componentName resource.Name,
			grabPose *referenceframe.PoseInFrame,
			worldState *commonpb.WorldState,
		) (bool, error) {
			return false, passedErr
		}
		passedErr = errors.New("fake GetPose error")
		injectMS.GetPoseFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destinationFrame string,
		) (*referenceframe.PoseInFrame, error) {
			return nil, passedErr
		}

		resp, err := client2.Move(context.Background(), resourceName, grabPose, &commonpb.WorldState{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, resp, test.ShouldEqual, false)
		_, err = client2.GetPose(context.Background(), arm.Named("arm1"), "foo")
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, utils.TryClose(context.Background(), client2), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectMS := &inject.MotionService{}
	omMap := map[resource.Name]interface{}{
		motion.Name: injectMS,
	}
	server, err := newServer(omMap)
	test.That(t, err, test.ShouldBeNil)
	servicepb.RegisterMotionServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := motion.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := motion.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
