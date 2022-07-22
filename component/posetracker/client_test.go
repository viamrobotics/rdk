package posetracker_test

import (
	"context"
	"errors"
	"math"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/posetracker"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

const (
	zeroPoseBody     = "zeroBody"
	nonZeroPoseBody  = "body2"
	nonZeroPoseBody2 = "body3"
	otherBodyFrame   = "bodyFrame2"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	workingPT := &inject.PoseTracker{}
	failingPT := &inject.PoseTracker{}

	pose := spatialmath.NewPoseFromOrientation(r3.Vector{X: 2, Y: 4, Z: 6}, &spatialmath.R4AA{Theta: math.Pi, RX: 0, RY: 0, RZ: 1})
	pose2 := spatialmath.NewPoseFromOrientation(r3.Vector{X: 1, Y: 2, Z: 3}, &spatialmath.R4AA{Theta: math.Pi, RX: 0, RY: 0, RZ: 1})
	zeroPose := spatialmath.NewZeroPose()
	allBodiesToPoseInFrames := posetracker.BodyToPoseInFrame{
		zeroPoseBody:     referenceframe.NewPoseInFrame(bodyFrame, zeroPose),
		nonZeroPoseBody:  referenceframe.NewPoseInFrame(bodyFrame, pose),
		nonZeroPoseBody2: referenceframe.NewPoseInFrame(otherBodyFrame, pose2),
	}
	poseTester := func(
		t *testing.T, receivedPoseInFrames posetracker.BodyToPoseInFrame,
		bodyName string,
	) {
		t.Helper()
		poseInFrame, ok := receivedPoseInFrames[bodyName]
		test.That(t, ok, test.ShouldBeTrue)
		expectedPoseInFrame := allBodiesToPoseInFrames[bodyName]
		test.That(t, poseInFrame.FrameName(), test.ShouldEqual, expectedPoseInFrame.FrameName())
		poseEqualToExpected := spatialmath.PoseAlmostEqual(poseInFrame.Pose(), expectedPoseInFrame.Pose())
		test.That(t, poseEqualToExpected, test.ShouldBeTrue)
	}

	workingPT.GetPosesFunc = func(ctx context.Context, bodyNames []string) (
		posetracker.BodyToPoseInFrame, error,
	) {
		return allBodiesToPoseInFrames, nil
	}

	failingPT.GetPosesFunc = func(ctx context.Context, bodyNames []string) (
		posetracker.BodyToPoseInFrame, error,
	) {
		return nil, errors.New("failure to get poses")
	}

	resourceMap := map[resource.Name]interface{}{
		posetracker.Named(workingPTName): workingPT,
		posetracker.Named(failingPTName): failingPT,
	}
	ptSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(posetracker.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, ptSvc)

	workingPT.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, ptSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	workingPTClient := posetracker.NewClientFromConn(context.Background(), conn, workingPTName, logger)

	t.Run("client tests for working pose tracker", func(t *testing.T) {
		bodyToPoseInFrame, err := workingPTClient.GetPoses(
			context.Background(), []string{zeroPoseBody, nonZeroPoseBody})
		test.That(t, err, test.ShouldBeNil)

		// Do
		resp, err := workingPTClient.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		poseTester(t, bodyToPoseInFrame, zeroPoseBody)
		poseTester(t, bodyToPoseInFrame, nonZeroPoseBody)
		poseTester(t, bodyToPoseInFrame, nonZeroPoseBody2)
	})

	t.Run("dialed client tests for working pose tracker", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, workingPTName, logger)
		workingPTDialedClient, ok := client.(posetracker.PoseTracker)
		test.That(t, ok, test.ShouldBeTrue)
		bodyToPoseInFrame, err := workingPTDialedClient.GetPoses(context.Background(), []string{})
		test.That(t, err, test.ShouldBeNil)

		poseTester(t, bodyToPoseInFrame, nonZeroPoseBody2)
		poseTester(t, bodyToPoseInFrame, nonZeroPoseBody)
		poseTester(t, bodyToPoseInFrame, zeroPoseBody)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for failing pose tracker", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingPTDialedClient := posetracker.NewClientFromConn(
			context.Background(), conn, failingPTName, logger,
		)

		bodyToPoseInFrame, err := failingPTDialedClient.GetPoses(context.Background(), []string{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, bodyToPoseInFrame, test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, conn.Close(), test.ShouldBeNil)
}
