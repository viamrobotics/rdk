package vision_test

import (
	"context"
	"image"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/objectdetection"
)

var visName1 = vision.Named("vision1")

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	srv := &inject.VisionService{}
	srv.DetectionsFunc = func(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
		det1 := objectdetection.NewDetection(image.Rect(5, 10, 15, 20), 0.5, "yes")
		return []objectdetection.Detection{det1}, nil
	}
	srv.DetectionsFromCameraFunc = func(
		ctx context.Context,
		camName string,
		extra map[string]interface{},
	) ([]objectdetection.Detection, error) {
		det1 := objectdetection.NewDetection(image.Rect(0, 0, 10, 20), 0.8, "camera")
		return []objectdetection.Detection{det1}, nil
	}
	test.That(t, err, test.ShouldBeNil)
	m := map[resource.Name]vision.Service{
		vision.Named(testVisionServiceName): srv,
	}
	svc, err := resource.NewAPIResourceCollection(vision.API, m)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[vision.Service](vision.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, svc), test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("get detections from img", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := vision.NewClientFromConn(context.Background(), conn, "", vision.Named(testVisionServiceName), logger)
		test.That(t, err, test.ShouldBeNil)

		dets, err := client.Detections(context.Background(), &image.RGBA{}, nil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, dets, test.ShouldNotBeNil)
		test.That(t, dets[0].Label(), test.ShouldResemble, "yes")
		test.That(t, dets[0].Score(), test.ShouldEqual, 0.5)
		box := dets[0].BoundingBox()
		test.That(t, box.Min, test.ShouldResemble, image.Point{5, 10})
		test.That(t, box.Max, test.ShouldResemble, image.Point{15, 20})

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("get detections from cam", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := vision.NewClientFromConn(context.Background(), conn, "", vision.Named(testVisionServiceName), logger)
		test.That(t, err, test.ShouldBeNil)

		dets, err := client.DetectionsFromCamera(context.Background(), "fake_cam", nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dets, test.ShouldHaveLength, 1)
		test.That(t, dets[0].Label(), test.ShouldEqual, "camera")
		test.That(t, dets[0].Score(), test.ShouldEqual, 0.8)
		box := dets[0].BoundingBox()
		test.That(t, box.Min, test.ShouldResemble, image.Point{0, 0})
		test.That(t, box.Max, test.ShouldResemble, image.Point{10, 20})

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestInjectedServiceClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectVision := &inject.VisionService{}
	osMap := map[resource.Name]vision.Service{
		visName1: injectVision,
	}
	svc, err := resource.NewAPIResourceCollection(vision.API, osMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[vision.Service](vision.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, svc), test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Do Command", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient, err := vision.NewClientFromConn(context.Background(), conn, "", vision.Named(testVisionServiceName), logger)
		test.That(t, err, test.ShouldBeNil)
		injectVision.DoCommandFunc = testutils.EchoFunc
		resp, err := workingDialedClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, workingDialedClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
