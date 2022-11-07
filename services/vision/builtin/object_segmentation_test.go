package builtin_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/services/vision/builtin"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
)

const (
	testVisionServiceName = "vision1"
)

func TestObjectSegmentationFailures(t *testing.T) {
	cfgService := config.Service{}
	logger := golog.NewTestLogger(t)

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		return nil, rdkutils.NewResourceNotFoundError(n)
	}
	// fails on not finding the service
	_, err := vision.FromRobot(r, testVisionServiceName)
	test.That(t, err, test.ShouldBeError, rdkutils.NewResourceNotFoundError(vision.Named(testVisionServiceName)))

	// fails on not finding camera
	obs, err := builtin.NewBuiltIn(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = obs.GetObjectPointClouds(context.Background(), "fakeCamera", "", map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)

	// fails since camera cannot generate point clouds (no depth in image)
	r = &inject.Robot{}
	_cam := &simpleSource{}
	cam, err := camera.NewFromReader(context.Background(), _cam, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("fakeCamera")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		switch n.Name {
		case "fakeCamera":
			return cam, nil
		default:
			return nil, rdkutils.NewResourceNotFoundError(n)
		}
	}

	obs, err = builtin.NewBuiltIn(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	params := config.AttributeMap{
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
	cfgService := config.Service{}
	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}
	_cam := &cloudSource{}
	cam, err := camera.NewFromReader(context.Background(), _cam, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("fakeCamera")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		switch n.Name {
		case "fakeCamera":
			return cam, nil
		default:
			return nil, rdkutils.NewResourceNotFoundError(n)
		}
	}

	obs, err := builtin.NewBuiltIn(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	// add segmenter to service
	params := config.AttributeMap{
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

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	return r, svc1
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := vision.FromRobot(r, testVisionServiceName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	result, err := svc.GetObjectPointClouds(context.Background(), "", "", map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 2)
	test.That(t, svc1.timesCalled, test.ShouldEqual, 1)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return "not object segmentation", nil
	}

	svc, err = vision.FromRobot(r, testVisionServiceName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, vision.NewUnimplementedInterfaceError("string"))
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return nil, rdkutils.NewResourceNotFoundError(name)
	}

	svc, err = vision.FromRobot(r, testVisionServiceName)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	test.That(t, svc, test.ShouldBeNil)
}

type mock struct {
	vision.Service
	timesCalled int
}

func (m *mock) GetObjectPointClouds(ctx context.Context,
	cameraName string,
	segmenterName string,
	extra map[string]interface{},
) ([]*viz.Object, error) {
	m.timesCalled++
	return []*viz.Object{viz.NewEmptyObject(), viz.NewEmptyObject()}, nil
}

func TestFullClientServerLoop(t *testing.T) {
	cfgService := config.Service{}
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	// create the robot, camera, and service
	r := &inject.Robot{}
	_cam := &cloudSource{}
	cam, err := camera.NewFromReader(context.Background(), _cam, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("fakeCamera")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		switch n.Name {
		case "fakeCamera":
			return cam, nil
		default:
			return nil, rdkutils.NewResourceNotFoundError(n)
		}
	}
	oss, err := builtin.NewBuiltIn(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	osMap := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): oss,
	}
	svc, err := subtype.New(osMap)
	test.That(t, err, test.ShouldBeNil)
	// test the server/client
	resourceSubtype := registry.ResourceSubtypeLookup(vision.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	client := vision.NewClientFromConn(context.Background(), conn, testVisionServiceName, logger)

	test.That(t, err, test.ShouldBeNil)
	expectedLabel := "test_label"
	params := config.AttributeMap{
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

	test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}
