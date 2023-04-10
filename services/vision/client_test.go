package vision_test

import (
	"context"
	"image"
	"net"
	"testing"

	"github.com/edaniels/golog"
	servicepb "go.viam.com/api/service/vision/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/objectdetection"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{}
	srv := &inject.VisionService{}
	srv.DetectionsFunc = func(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
		det1 := objectdetection.NewDetection(image.Rect(5, 10, 15, 20), 0.5, "yes")
		return []objectdetection.Detection{det1}, nil
	}
	srv.DetectionsFromCameraFunc = func(ctx context.Context, camName string, extra map[string]interface{}) ([]objectdetection.Detection, error) {
		det1 := objectdetection.NewDetection(image.Rect(0, 0, 10, 20), 0.8, "camera")
		return []objectdetection.Detection{det1}, nil
	}
	test.That(t, err, test.ShouldBeNil)
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): srv,
		camera.Named("fake_cam"):            cam,
	}
	svc, err := subtype.New(m)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(vision.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

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
		client := vision.NewClientFromConn(context.Background(), conn, testVisionServiceName, logger)

		dets, err := client.Detections(context.Background(), &image.RGBA{}, nil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, dets, test.ShouldNotBeNil)
		test.That(t, dets[0].Label(), test.ShouldResemble, "yes")
		test.That(t, dets[0].Score(), test.ShouldEqual, 0.5)
		box := dets[0].BoundingBox()
		test.That(t, box.Min, test.ShouldResemble, image.Point{5, 10})
		test.That(t, box.Max, test.ShouldResemble, image.Point{15, 20})

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("get detections from cam", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := vision.NewClientFromConn(context.Background(), conn, testVisionServiceName, logger)

		dets, err := client.DetectionsFromCamera(context.Background(), "fake_cam", nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dets, test.ShouldHaveLength, 1)
		test.That(t, dets[0].Label(), test.ShouldEqual, "camera")
		test.That(t, dets[0].Score(), test.ShouldEqual, 0.8)
		box := dets[0].BoundingBox()
		test.That(t, box.Min, test.ShouldResemble, image.Point{0, 0})
		test.That(t, box.Max, test.ShouldResemble, image.Point{10, 20})

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestInjectedServiceClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectVision := &inject.VisionService{}
	osMap := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): injectVision,
	}
	svc, err := subtype.New(osMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(vision.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Do Command", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient := vision.NewClientFromConn(context.Background(), conn, testVisionServiceName, logger)
		injectVision.DoCommandFunc = generic.EchoFunc
		resp, err := workingDialedClient.DoCommand(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		test.That(t, utils.TryClose(context.Background(), workingDialedClient), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectODS := &inject.VisionService{}
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): injectODS,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	servicepb.RegisterVisionServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := vision.NewClientFromConn(ctx, conn1, testVisionServiceName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := vision.NewClientFromConn(ctx, conn2, testVisionServiceName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}
