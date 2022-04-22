package objectdetection_test

import (
	"context"
	"image"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/objectdetection/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectdetection"
	"go.viam.com/rdk/services/objectsegmentation"
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

	r := buildRobotWithFakeCamera(t)
	srv, err := objectdetection.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	segsrv, err := objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	m := map[resource.Name]interface{}{
		objectdetection.Name:    srv,
		objectsegmentation.Name: segsrv,
	}
	svc, err := subtype.New(m)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(objectdetection.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)
	resourceSubtype = registry.ResourceSubtypeLookup(objectsegmentation.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = objectdetection.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("working client", func(t *testing.T) {
		client, err := objectdetection.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		names, err := client.DetectorNames(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldContain, "detect_red")

		segClient, err := objectsegmentation.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		segNames, err := segClient.GetSegmenters(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, segNames, test.ShouldContain, "detect_red")

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, utils.TryClose(context.Background(), segClient), test.ShouldBeNil)
	})

	t.Run("add detector", func(t *testing.T) {
		client, err := objectdetection.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		cfg := objectdetection.Config{
			Name: "new_detector",
			Type: "color",
			Parameters: config.AttributeMap{
				"detect_color": "#112233",
				"tolerance":    0.9,
				"segment_size": 3333333,
			},
		}
		// success
		ok, err := client.AddDetector(context.Background(), cfg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeTrue)

		names, err := client.DetectorNames(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldContain, "detect_red")
		test.That(t, names, test.ShouldContain, "new_detector")
		// check that it also also added to the segmenter
		segClient, err := objectsegmentation.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		segNames, err := segClient.GetSegmenters(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, segNames, test.ShouldContain, "detect_red")
		test.That(t, segNames, test.ShouldContain, "new_detector")
		// failure - tries to add a detector again
		ok, err = client.AddDetector(context.Background(), cfg)
		test.That(t, err.Error(), test.ShouldContainSubstring, "trying to register two detectors with the same name")
		test.That(t, ok, test.ShouldBeFalse)

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, utils.TryClose(context.Background(), segClient), test.ShouldBeNil)
	})
	t.Run("get detections", func(t *testing.T) {
		client, err := objectdetection.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		dets, err := client.Detect(context.Background(), "fake_cam", "detect_red")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dets, test.ShouldHaveLength, 1)
		test.That(t, dets[0].Label(), test.ShouldEqual, "red")
		test.That(t, dets[0].Score(), test.ShouldEqual, 1.0)
		box := dets[0].BoundingBox()
		test.That(t, box.Min, test.ShouldResemble, image.Point{110, 288})
		test.That(t, box.Max, test.ShouldResemble, image.Point{183, 349})
		// failure - no such camera
		_, err = client.Detect(context.Background(), "no_camera", "detect_red")
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectODS := &inject.ObjectDetectionService{}
	m := map[resource.Name]interface{}{
		objectdetection.Name: injectODS,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	servicepb.RegisterObjectDetectionServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := objectdetection.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := objectdetection.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
