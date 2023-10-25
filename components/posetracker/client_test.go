package posetracker_test

import (
	"context"
	"math"
	"net"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/posetracker"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	zeroPoseBody     = "zeroBody"
	nonZeroPoseBody  = "body2"
	nonZeroPoseBody2 = "body3"
	otherBodyFrame   = "bodyFrame2"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	workingPT := &inject.PoseTracker{}
	failingPT := &inject.PoseTracker{}

	pose := spatialmath.NewPose(r3.Vector{X: 2, Y: 4, Z: 6}, &spatialmath.R4AA{Theta: math.Pi, RX: 0, RY: 0, RZ: 1})
	pose2 := spatialmath.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatialmath.R4AA{Theta: math.Pi, RX: 0, RY: 0, RZ: 1})
	zeroPose := spatialmath.NewZeroPose()
	allBodiesToPoseInFrames := posetracker.BodyToPoseInFrame{
		zeroPoseBody:     referenceframe.NewPoseInFrame(bodyFrame, zeroPose),
		nonZeroPoseBody:  referenceframe.NewPoseInFrame(bodyFrame, pose),
		nonZeroPoseBody2: referenceframe.NewPoseInFrame(otherBodyFrame, pose2),
	}
	var extraOptions map[string]interface{}
	poseTester := func(
		t *testing.T, receivedPoseInFrames posetracker.BodyToPoseInFrame,
		bodyName string,
	) {
		t.Helper()
		poseInFrame, ok := receivedPoseInFrames[bodyName]
		test.That(t, ok, test.ShouldBeTrue)
		expectedPoseInFrame := allBodiesToPoseInFrames[bodyName]
		test.That(t, poseInFrame.Parent(), test.ShouldEqual, expectedPoseInFrame.Parent())
		poseEqualToExpected := spatialmath.PoseAlmostEqual(poseInFrame.Pose(), expectedPoseInFrame.Pose())
		test.That(t, poseEqualToExpected, test.ShouldBeTrue)
	}

	workingPT.PosesFunc = func(ctx context.Context, bodyNames []string, extra map[string]interface{}) (
		posetracker.BodyToPoseInFrame, error,
	) {
		extraOptions = extra
		return allBodiesToPoseInFrames, nil
	}

	failingPT.PosesFunc = func(ctx context.Context, bodyNames []string, extra map[string]interface{}) (
		posetracker.BodyToPoseInFrame, error,
	) {
		return nil, errPoseFailed
	}

	resourceMap := map[resource.Name]posetracker.PoseTracker{
		posetracker.Named(workingPTName): workingPT,
		posetracker.Named(failingPTName): failingPT,
	}
	ptSvc, err := resource.NewAPIResourceCollection(posetracker.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[posetracker.PoseTracker](posetracker.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, ptSvc), test.ShouldBeNil)

	workingPT.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	workingPTClient, err := posetracker.NewClientFromConn(context.Background(), conn, "", posetracker.Named(workingPTName), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for working pose tracker", func(t *testing.T) {
		bodyToPoseInFrame, err := workingPTClient.Poses(
			context.Background(), []string{zeroPoseBody, nonZeroPoseBody}, map[string]interface{}{"foo": "Poses"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "Poses"})

		// DoCommand
		resp, err := workingPTClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		poseTester(t, bodyToPoseInFrame, zeroPoseBody)
		poseTester(t, bodyToPoseInFrame, nonZeroPoseBody)
		poseTester(t, bodyToPoseInFrame, nonZeroPoseBody2)
	})

	t.Run("dialed client tests for working pose tracker", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := resourceAPI.RPCClient(context.Background(), conn, "", posetracker.Named(workingPTName), logger)
		test.That(t, err, test.ShouldBeNil)
		bodyToPoseInFrame, err := client.Poses(context.Background(), []string{}, map[string]interface{}{"foo": "PosesDialed"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{"foo": "PosesDialed"})

		poseTester(t, bodyToPoseInFrame, nonZeroPoseBody2)
		poseTester(t, bodyToPoseInFrame, nonZeroPoseBody)
		poseTester(t, bodyToPoseInFrame, zeroPoseBody)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for failing pose tracker", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingPTDialedClient, err := posetracker.NewClientFromConn(context.Background(), conn, "", posetracker.Named(failingPTName), logger)
		test.That(t, err, test.ShouldBeNil)

		bodyToPoseInFrame, err := failingPTDialedClient.Poses(context.Background(), []string{}, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPoseFailed.Error())
		test.That(t, bodyToPoseInFrame, test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, conn.Close(), test.ShouldBeNil)
}
