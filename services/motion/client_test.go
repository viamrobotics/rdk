package motion_test

import (
	"context"
	"math"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gripper"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testMotionServiceName  = motion.Named("motion1")
	testMotionServiceName2 = motion.Named("motion2")
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectMS := &inject.MotionService{}
	omMap := map[resource.Name]resource.Resource{
		testMotionServiceName: injectMS,
	}
	svc, err := subtype.New(mlmodel.Subtype, omMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype, ok := registry.ResourceSubtypeLookup(motion.Subtype)
	test.That(t, ok, test.ShouldBeTrue)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)
	grabPose := referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())
	resourceName := gripper.Named("fake")
	test.That(t, err, test.ShouldBeNil)
	ctx := context.Background()

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
	t.Run("motion client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)

		test.That(t, err, test.ShouldBeNil)

		client, err := motion.NewClientFromConn(context.Background(), conn, testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		receivedTransforms := make(map[string]*referenceframe.LinkInFrame)
		success := true
		injectMS.MoveFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destination *referenceframe.PoseInFrame,
			worldState *referenceframe.WorldState,
			constraints *servicepb.Constraints,
			extra map[string]interface{},
		) (bool, error) {
			return success, nil
		}
		injectMS.GetPoseFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destinationFrame string,
			supplementalTransforms []*referenceframe.LinkInFrame,
			extra map[string]interface{},
		) (*referenceframe.PoseInFrame, error) {
			for _, tf := range supplementalTransforms {
				receivedTransforms[tf.Name()] = tf
			}
			return referenceframe.NewPoseInFrame(
				destinationFrame+componentName.Name, spatialmath.NewPoseFromPoint(r3.Vector{1, 2, 3})), nil
		}

		result, err := client.Move(ctx, resourceName, grabPose, &referenceframe.WorldState{}, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldEqual, success)

		testPose := spatialmath.NewPose(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)

		transforms := []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame("arm1", testPose, "frame1", nil),
			referenceframe.NewLinkInFrame("frame1", testPose, "frame2", nil),
		}

		tfMap := make(map[string]*referenceframe.LinkInFrame)
		for _, tf := range transforms {
			tfMap[tf.Name()] = tf
		}
		poseResult, err := client.GetPose(context.Background(), arm.Named("arm1"), "foo", transforms, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, poseResult.Parent(), test.ShouldEqual, "fooarm1")
		test.That(t, poseResult.Pose().Point().X, test.ShouldEqual, 1)
		test.That(t, poseResult.Pose().Point().Y, test.ShouldEqual, 2)
		test.That(t, poseResult.Pose().Point().Z, test.ShouldEqual, 3)
		for name, tf := range tfMap {
			receivedTf := receivedTransforms[name]
			test.That(t, tf.Name(), test.ShouldEqual, receivedTf.Name())
			test.That(t, tf.Parent(), test.ShouldEqual, receivedTf.Parent())
			test.That(t, spatialmath.PoseAlmostEqual(tf.Pose(), receivedTf.Pose()), test.ShouldBeTrue)
		}
		test.That(t, receivedTransforms, test.ShouldNotBeNil)

		// DoCommand
		injectMS.DoCommandFunc = testutils.EchoFunc
		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	// broken
	t.Run("motion client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := resourceSubtype.RPCClient(context.Background(), conn, testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)
		client2, ok := client.(motion.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("fake move error")
		injectMS.MoveFunc = func(
			ctx context.Context,
			componentName resource.Name,
			grabPose *referenceframe.PoseInFrame,
			worldState *referenceframe.WorldState,
			constraints *servicepb.Constraints,
			extra map[string]interface{},
		) (bool, error) {
			return false, passedErr
		}
		passedErr = errors.New("fake GetPose error")
		injectMS.GetPoseFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destinationFrame string,
			supplementalTransform []*referenceframe.LinkInFrame,
			extra map[string]interface{},
		) (*referenceframe.PoseInFrame, error) {
			return nil, passedErr
		}

		resp, err := client2.Move(ctx, resourceName, grabPose, &referenceframe.WorldState{}, nil, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, resp, test.ShouldEqual, false)
		_, err = client2.GetPose(context.Background(), arm.Named("arm1"), "foo", nil, map[string]interface{}{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, utils.TryClose(context.Background(), client2), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
