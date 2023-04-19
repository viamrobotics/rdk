package vision_test

import (
	"context"
	"image"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/camera"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/services/vision/builtin"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

var (
	visName1                  = vision.Named("vision1")
	RadiusClusteringSegmenter = "radius_clustering"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	r, err := buildRobotWithFakeCamera(t)
	test.That(t, err, test.ShouldBeNil)
	visName := vision.FindFirstName(r)
	srv, err := vision.FromRobot(r, visName)
	test.That(t, err, test.ShouldBeNil)
	m := map[resource.Name]resource.Resource{
		vision.Named(visName): srv,
	}
	svc, err := subtype.New(vision.Subtype, m)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype, ok := registry.ResourceSubtypeLookup(vision.Subtype)
	test.That(t, ok, test.ShouldBeTrue)
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

	t.Run("model schema", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(visName), logger)
		test.That(t, err, test.ShouldBeNil)

		params, err := client.GetModelParameterSchema(context.Background(), builtin.RCSegmenter, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		parameterNames := params.Definitions["RadiusClusteringConfig"].Required
		test.That(t, parameterNames, test.ShouldContain, "min_points_in_plane")
		test.That(t, parameterNames, test.ShouldContain, "min_points_in_segment")
		test.That(t, parameterNames, test.ShouldContain, "clustering_radius_mm")
		test.That(t, parameterNames, test.ShouldContain, "mean_k_filtering")

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("detector names", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(visName), logger)
		test.That(t, err, test.ShouldBeNil)

		names, err := client.DetectorNames(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldContain, "detect_red")

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("add detector", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(visName), logger)
		test.That(t, err, test.ShouldBeNil)

		cfg := vision.VisModelConfig{
			Name: "new_detector",
			Type: "color_detector",
			Parameters: rutils.AttributeMap{
				"detect_color":      "#112233",
				"hue_tolerance_pct": 0.9,
				"value_cutoff_pct":  0.2,
				"segment_size_px":   3333333,
			},
		}
		// success
		err = client.AddDetector(context.Background(), cfg, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		names, err := client.DetectorNames(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldContain, "detect_red")
		test.That(t, names, test.ShouldContain, "new_detector")
		// tries to add a detector again
		err = client.AddDetector(context.Background(), cfg, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("get detections from cam", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(visName), logger)
		test.That(t, err, test.ShouldBeNil)

		dets, err := client.DetectionsFromCamera(context.Background(), "fake_cam", "detect_red", map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dets, test.ShouldHaveLength, 1)
		test.That(t, dets[0].Label(), test.ShouldEqual, "red")
		test.That(t, dets[0].Score(), test.ShouldEqual, 1.0)
		box := dets[0].BoundingBox()
		test.That(t, box.Min, test.ShouldResemble, image.Point{110, 288})
		test.That(t, box.Max, test.ShouldResemble, image.Point{183, 349})
		// failure - no such camera
		_, err = client.DetectionsFromCamera(context.Background(), "no_camera", "detect_red", map[string]interface{}{})
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("get detections from img", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(visName), logger)
		test.That(t, err, test.ShouldBeNil)

		img, _ := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
		modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
		cfg := vision.VisModelConfig{
			Name: "test", Type: "tflite_detector",
			Parameters: rutils.AttributeMap{
				"model_path":  modelLoc,
				"label_path":  "",
				"num_threads": 2,
			},
		}
		err = client.AddDetector(context.Background(), cfg, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		dets, err := client.Detections(context.Background(), img, "test", map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, dets, test.ShouldNotBeNil)
		test.That(t, dets[0].Label(), test.ShouldResemble, "17")
		test.That(t, dets[0].Score(), test.ShouldBeGreaterThan, 0.78)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("segmenters", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(visName), logger)
		test.That(t, err, test.ShouldBeNil)

		names, err := client.SegmenterNames(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldNotContain, "new_segmenter")

		cfg := vision.VisModelConfig{
			Name: "new_segmenter",
			Type: string(builtin.RCSegmenter),
			Parameters: rutils.AttributeMap{
				"min_points_in_plane":   100,
				"min_points_in_segment": 3,
				"clustering_radius_mm":  5.,
				"mean_k_filtering":      10.,
			},
		}

		err = client.AddSegmenter(context.Background(), cfg, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		names, err = client.SegmenterNames(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldContain, "new_segmenter")

		err = client.RemoveClassifier(context.Background(), "new_segmenter", map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		names, err = client.ClassifierNames(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldNotContain, "new_segmenter")

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("add/remove/classifiernames", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(visName), logger)
		test.That(t, err, test.ShouldBeNil)

		names, err := client.ClassifierNames(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldBeEmpty)

		cfg := vision.VisModelConfig{
			Name: "new_class",
			Type: "tflite_classifier",
			Parameters: rutils.AttributeMap{
				"model_path":  artifact.MustPath("vision/tflite/effnet0.tflite"),
				"label_path":  "",
				"num_threads": 1,
			},
		}
		cfg2 := vision.VisModelConfig{
			Name: "better_class",
			Type: "tflite_classifier",
			Parameters: rutils.AttributeMap{
				"model_path":  artifact.MustPath("vision/tflite/effnet0.tflite"),
				"label_path":  "",
				"num_threads": 2,
			},
		}

		// success
		err = client.AddClassifier(context.Background(), cfg, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		names, err = client.ClassifierNames(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldContain, "new_class")
		names, err = client.DetectorNames(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldNotContain, "new_class")

		err = client.AddClassifier(context.Background(), cfg2, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		err = client.RemoveClassifier(context.Background(), "new_class", map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		names, err = client.ClassifierNames(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldNotContain, "new_class")
		test.That(t, names, test.ShouldContain, "better_class")

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("get classifications from cam", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(visName), logger)
		test.That(t, err, test.ShouldBeNil)

		classifs, err := client.ClassificationsFromCamera(context.Background(), "fake_cam2", "better_class", 3, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, classifs, test.ShouldNotBeNil)
		test.That(t, classifs, test.ShouldHaveLength, 3)
		test.That(t, classifs[0].Label(), test.ShouldResemble, "291")
		test.That(t, classifs[0].Score(), test.ShouldBeGreaterThan, 0.82)

		// failure - no such camera
		_, err = client.ClassificationsFromCamera(context.Background(), "no_camera", "better_class", 3, map[string]interface{}{})
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("get classifications from img", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(visName), logger)
		test.That(t, err, test.ShouldBeNil)

		img, _ := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))

		classifs, err := client.Classifications(context.Background(), img, "better_class", 5, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, classifs, test.ShouldNotBeNil)
		test.That(t, classifs, test.ShouldHaveLength, 5)
		test.That(t, classifs[0].Label(), test.ShouldResemble, "291")
		test.That(t, classifs[0].Score(), test.ShouldBeGreaterThan, 0.82)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestInjectedServiceClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectVision := &inject.VisionService{}
	osMap := map[resource.Name]resource.Resource{
		visName1: injectVision,
	}
	svc, err := subtype.New(vision.Subtype, osMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype, ok := registry.ResourceSubtypeLookup(vision.Subtype)
	test.That(t, ok, test.ShouldBeTrue)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("dialed client test config for working vision service", func(t *testing.T) {
		var extraOptions map[string]interface{}
		injectVision.SegmenterNamesFunc = func(ctx context.Context, extra map[string]interface{}) ([]string, error) {
			extraOptions = extra
			return []string{RadiusClusteringSegmenter}, nil
		}

		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient, err := vision.NewClientFromConn(context.Background(), conn, visName1, logger)
		test.That(t, err, test.ShouldBeNil)
		extra := map[string]interface{}{"foo": "SegmenterNames"}
		segmenterNames, err := workingDialedClient.SegmenterNames(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, segmenterNames, test.ShouldHaveLength, 1)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		// DoCommand
		injectVision.DoCommandFunc = testutils.EchoFunc
		resp, err := workingDialedClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, workingDialedClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	t.Run("test segmentation", func(t *testing.T) {
		params := rutils.AttributeMap{
			"min_points_in_plane":   100,
			"min_points_in_segment": 3,
			"clustering_radius_mm":  5.,
			"mean_k_filtering":      10.,
		}
		segmenter, err := segmentation.NewRadiusClustering(params)
		test.That(t, err, test.ShouldBeNil)
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := vision.NewClientFromConn(context.Background(), conn, visName1, logger)
		test.That(t, err, test.ShouldBeNil)

		camSource := &cloudSource{}
		injSrc, err := camera.NewVideoSourceFromReader(context.Background(), camSource, nil, camera.ColorStream)
		test.That(t, err, test.ShouldBeNil)

		var extraOptions map[string]interface{}
		injectVision.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName, segmenterName string, extra map[string]interface{},
		) ([]*viz.Object, error) {
			extraOptions = extra
			segments, err := segmenter(ctx, injSrc)
			if err != nil {
				return nil, err
			}
			return segments, nil
		}

		extra := map[string]interface{}{"foo": "GetObjectPointClouds"}
		segs, err := client.GetObjectPointClouds(context.Background(), "cloud_cam", RadiusClusteringSegmenter, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(segs), test.ShouldEqual, 2)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		expectedBoxes := makeExpectedBoxes(t)
		for _, seg := range segs {
			box, err := pointcloud.BoundingBoxFromPointCloud(seg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, box, test.ShouldNotBeNil)
			test.That(t, box.AlmostEqual(expectedBoxes[0]) || box.AlmostEqual(expectedBoxes[1]), test.ShouldBeTrue)
		}
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
