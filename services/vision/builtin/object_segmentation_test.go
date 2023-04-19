package builtin_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/camera"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/services/vision/builtin"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
)

const (
	testVisionServiceName = "vision1"
)

func TestObjectSegmentationFailures(t *testing.T) {
	cfgService := resource.Config{
		ConvertedAttributes: &vision.Config{},
	}
	logger := golog.NewTestLogger(t)

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		return nil, resource.NewNotFoundError(n)
	}
	// fails on not finding the service
	_, err := vision.FromRobot(r, testVisionServiceName)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(vision.Named(testVisionServiceName)))

	// fails on not finding camera
	obs, err := builtin.NewBuiltIn(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = obs.GetObjectPointClouds(context.Background(), "fakeCamera", "", map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)

	// fails since camera cannot generate point clouds (no depth in image)
	r = &inject.Robot{}
	camSource := &simpleSource{}
	src, err := camera.NewVideoSourceFromReader(context.Background(), camSource, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("fakeCamera")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		switch n.Name {
		case "fakeCamera":
			return camera.FromVideoSource(camera.Named("fakeCamera"), src), nil
		default:
			return nil, resource.NewNotFoundError(n)
		}
	}

	obs, err = builtin.NewBuiltIn(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	params := rutils.AttributeMap{
		"min_points_in_plane":   100,
		"min_points_in_segment": 3,
		"clustering_radius_mm":  5.,
		"mean_k_filtering":      10.,
	}

	err = obs.AddSegmenter(
		context.Background(),
		vision.VisModelConfig{builtin.RadiusClusteringSegmenter, string(builtin.RCSegmenter), params},
		map[string]interface{}{},
	)
	test.That(t, err, test.ShouldBeNil)
	_, err = obs.GetObjectPointClouds(context.Background(), "fakeCamera", builtin.RadiusClusteringSegmenter, map[string]interface{}{})
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)
}

func TestGetObjectPointClouds(t *testing.T) {
	cfgService := resource.Config{
		ConvertedAttributes: &vision.Config{},
	}
	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}
	camSource := &cloudSource{}
	src, err := camera.NewVideoSourceFromReader(context.Background(), camSource, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("fakeCamera")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		switch n.Name {
		case "fakeCamera":
			return camera.FromVideoSource(camera.Named("fakeCamera"), src), nil
		default:
			return nil, resource.NewNotFoundError(n)
		}
	}

	obs, err := builtin.NewBuiltIn(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	// add segmenter to service
	params := rutils.AttributeMap{
		"min_points_in_plane":   100,
		"min_points_in_segment": 3,
		"clustering_radius_mm":  5.,
		"mean_k_filtering":      10.,
	}
	err = obs.AddSegmenter(
		context.Background(),
		vision.VisModelConfig{builtin.RadiusClusteringSegmenter, string(builtin.RCSegmenter), params},
		map[string]interface{}{},
	)
	test.That(t, err, test.ShouldBeNil)

	// see if it ws registered
	segmenterNames, err := obs.SegmenterNames(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segmenterNames, test.ShouldContain, builtin.RadiusClusteringSegmenter)

	// successfully get object point clouds
	segs, err := obs.GetObjectPointClouds(context.Background(), "fakeCamera", builtin.RadiusClusteringSegmenter, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segs), test.ShouldEqual, 2)

	expectedBoxes := makeExpectedBoxes(t)
	for _, seg := range segs {
		box, err := pointcloud.BoundingBoxFromPointCloud(seg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box, test.ShouldNotBeNil)
		test.That(t, box.AlmostEqual(expectedBoxes[0]) || box.AlmostEqual(expectedBoxes[1]), test.ShouldBeTrue)
	}

	// remove segmenter from service
	err = obs.RemoveSegmenter(context.Background(), builtin.RadiusClusteringSegmenter, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	segmenterNames, err = obs.SegmenterNames(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segmenterNames, test.ShouldHaveLength, 0)
}

func setupInjectRobot() (*inject.Robot, *int) {
	svc1 := inject.NewVisionService("something")
	var timesCalled int
	svc1.GetObjectPointCloudsFunc = func(ctx context.Context,
		cameraName string,
		segmenterName string,
		extra map[string]interface{},
	) ([]*viz.Object, error) {
		timesCalled++
		return []*viz.Object{viz.NewEmptyObject(), viz.NewEmptyObject()}, nil
	}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return svc1, nil
	}
	return r, &timesCalled
}

func TestFromRobot(t *testing.T) {
	r, timesCalled := setupInjectRobot()

	svc, err := vision.FromRobot(r, testVisionServiceName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	result, err := svc.GetObjectPointClouds(context.Background(), "", "", map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 2)
	test.That(t, *timesCalled, test.ShouldEqual, 1)

	notRight := testutils.NewUnimplementedResource(camera.Named("foo"))
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return notRight, nil
	}

	svc, err = vision.FromRobot(r, testVisionServiceName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, resource.TypeError[vision.Service](notRight))
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return nil, resource.NewNotFoundError(name)
	}

	svc, err = vision.FromRobot(r, testVisionServiceName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	test.That(t, svc, test.ShouldBeNil)
}

func TestFullClientServerLoop(t *testing.T) {
	cfgService := resource.Config{
		ConvertedAttributes: &vision.Config{},
	}
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	// create the robot, camera, and service
	r := &inject.Robot{}
	camSource := &cloudSource{}
	src, err := camera.NewVideoSourceFromReader(context.Background(), camSource, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("fakeCamera")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		switch n.Name {
		case "fakeCamera":
			return camera.FromVideoSource(camera.Named("fakeCamera"), src), nil
		default:
			return nil, resource.NewNotFoundError(n)
		}
	}
	oss, err := builtin.NewBuiltIn(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	osMap := map[resource.Name]resource.Resource{
		vision.Named(testVisionServiceName): oss,
	}
	svc, err := subtype.New(vision.Subtype, osMap)
	test.That(t, err, test.ShouldBeNil)
	// test the server/client
	resourceSubtype, ok := registry.ResourceSubtypeLookup(vision.Subtype)
	test.That(t, ok, test.ShouldBeTrue)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	client, err := vision.NewClientFromConn(context.Background(), conn, vision.Named(testVisionServiceName), logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, err, test.ShouldBeNil)
	expectedLabel := "test_label"
	params := rutils.AttributeMap{
		"min_points_in_plane":   100,
		"min_points_in_segment": 3,
		"clustering_radius_mm":  5.,
		"mean_k_filtering":      10.,
		"label":                 expectedLabel,
	}
	err = client.AddSegmenter(
		context.Background(),
		vision.VisModelConfig{builtin.RadiusClusteringSegmenter, string(builtin.RCSegmenter), params}, map[string]interface{}{},
	)
	test.That(t, err, test.ShouldBeNil)

	segs, err := client.GetObjectPointClouds(context.Background(), "fakeCamera", builtin.RadiusClusteringSegmenter, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segs), test.ShouldEqual, 2)

	expectedBoxes := makeExpectedBoxes(t)
	for _, seg := range segs {
		box, err := pointcloud.BoundingBoxFromPointCloudWithLabel(seg, seg.Geometry.Label())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box, test.ShouldNotBeNil)
		test.That(t, box.AlmostEqual(expectedBoxes[0]) || box.AlmostEqual(expectedBoxes[1]), test.ShouldBeTrue)
		test.That(t, box.Label(), test.ShouldEqual, expectedLabel)
	}

	test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}
