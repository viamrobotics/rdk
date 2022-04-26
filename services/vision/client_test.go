package vision_test

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
	"go.viam.com/rdk/pointcloud"
	servicepb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	r := buildRobotWithFakeCamera(t)
	srv, err := vision.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	m := map[resource.Name]interface{}{
		vision.Name: srv,
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
		_, err = vision.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("detector names", func(t *testing.T) {
		client, err := vision.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		names, err := client.DetectorNames(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldContain, "detect_red")

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	t.Run("add detector", func(t *testing.T) {
		client, err := vision.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		cfg := vision.DetectorConfig{
			Name: "new_detector",
			Type: "color",
			Parameters: config.AttributeMap{
				"detect_color": "#112233",
				"tolerance":    0.9,
				"segment_size": 3333333,
			},
		}
		// success
		err = client.AddDetector(context.Background(), cfg)
		test.That(t, err, test.ShouldBeNil)

		names, err := client.DetectorNames(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldContain, "detect_red")
		test.That(t, names, test.ShouldContain, "new_detector")
		// failure - tries to add a detector again
		err = client.AddDetector(context.Background(), cfg)
		test.That(t, err.Error(), test.ShouldContainSubstring, "trying to register two detectors with the same name")

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})
	t.Run("get detections", func(t *testing.T) {
		client, err := vision.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		dets, err := client.GetDetections(context.Background(), "fake_cam", "detect_red")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dets, test.ShouldHaveLength, 1)
		test.That(t, dets[0].Label(), test.ShouldEqual, "red")
		test.That(t, dets[0].Score(), test.ShouldEqual, 1.0)
		box := dets[0].BoundingBox()
		test.That(t, box.Min, test.ShouldResemble, image.Point{110, 288})
		test.That(t, box.Max, test.ShouldResemble, image.Point{183, 349})
		// failure - no such camera
		_, err = client.GetDetections(context.Background(), "no_camera", "detect_red")
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
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
		vision.Name: injectVision,
	}
	svc, err := subtype.New(osMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(vision.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("test segmentation", func(t *testing.T) {
		client, err := vision.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		injCam := &cloudSource{}

		injectVision.SegmenterParametersFunc = func(ctx context.Context, segmenterName string) ([]rdkutils.TypedName, error) {
			return rdkutils.JSONTags(segmentation.RadiusClusteringConfig{}), nil
		}
		injectVision.GetObjectPointCloudsFunc = func(ctx context.Context,
			cameraName string,
			segmenterName string,
			params config.AttributeMap,
		) ([]*viz.Object, error) {
			segments, err := segmentation.RadiusClustering(ctx, injCam, params)
			if err != nil {
				return nil, err
			}
			return segments, nil
		}
		injectVision.SegmenterNamesFunc = func(ctx context.Context) ([]string, error) {
			return []string{vision.RadiusClusteringSegmenter}, nil
		}

		segNames, err := client.SegmenterNames(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, segNames, test.ShouldHaveLength, 1)
		test.That(t, segNames[0], test.ShouldEqual, vision.RadiusClusteringSegmenter)

		paramNames, err := client.SegmenterParameters(context.Background(), segNames[0])
		test.That(t, err, test.ShouldBeNil)
		expParams := []rdkutils.TypedName{
			{"min_points_in_plane", "int"},
			{"min_points_in_segment", "int"},
			{"clustering_radius_mm", "float64"},
			{"mean_k_filtering", "int"},
		}
		test.That(t, paramNames, test.ShouldResemble, expParams)
		params := config.AttributeMap{
			paramNames[0].Name: 100,
			paramNames[1].Name: 3,
			paramNames[2].Name: 5.0,
			paramNames[3].Name: 10,
		}
		segs, err := client.GetObjectPointClouds(context.Background(), "", segNames[0], params)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(segs), test.ShouldEqual, 2)

		expectedBoxes := makeExpectedBoxes(t)
		for _, seg := range segs {
			box, err := pointcloud.BoundingBoxFromPointCloud(seg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, box, test.ShouldNotBeNil)
			test.That(t, box.AlmostEqual(expectedBoxes[0]) || box.AlmostEqual(expectedBoxes[1]), test.ShouldBeTrue)
		}

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectODS := &inject.VisionService{}
	m := map[resource.Name]interface{}{
		vision.Name: injectODS,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	servicepb.RegisterVisionServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := vision.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := vision.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
