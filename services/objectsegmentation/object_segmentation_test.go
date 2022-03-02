package objectsegmentation_test

import (
	"context"
	"image"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/objectsegmentation"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

var testPointCloud = []pointcloud.Point{
	pointcloud.NewBasicPoint(5, 5, 5),
	pointcloud.NewBasicPoint(5, 5, 6),
	pointcloud.NewBasicPoint(5, 5, 4),
	pointcloud.NewBasicPoint(-5, -5, 5),
	pointcloud.NewBasicPoint(-5, -5, 6),
	pointcloud.NewBasicPoint(-5, -5, 4),
}

func makeExpectedBoxes(t *testing.T) []spatialmath.Geometry {
	t.Helper()
	box1, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{X: -5, Y: -5, Z: 5}), r3.Vector{X: 0, Y: 0, Z: 2})
	test.That(t, err, test.ShouldBeNil)
	box2, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{X: 5, Y: 5, Z: 5}), r3.Vector{X: 0, Y: 0, Z: 2})
	test.That(t, err, test.ShouldBeNil)
	return []spatialmath.Geometry{box1, box2}
}

type simpleSource struct{}

func (s *simpleSource) Next(ctx context.Context) (image.Image, func(), error) {
	img := rimage.NewImage(100, 200)
	img.SetXY(20, 10, rimage.Red)
	return img, nil, nil
}

type cloudSource struct{}

func (c *cloudSource) Next(ctx context.Context) (image.Image, func(), error) {
	img := rimage.NewImage(100, 200)
	img.SetXY(20, 10, rimage.Red)
	return img, nil, nil
}

func (c *cloudSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	pcA := pointcloud.New()
	for _, pt := range testPointCloud {
		err := pcA.Set(pt)
		if err != nil {
			return nil, err
		}
	}
	return pcA, nil
}

func TestServiceFailures(t *testing.T) {
	cfgService := config.Service{}
	logger := golog.NewTestLogger(t)

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		return nil, rdkutils.NewResourceNotFoundError(n)
	}
	// fails on not finding the service
	_, err := objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rdkutils.NewResourceNotFoundError(objectsegmentation.Name))

	// fails on not finding camera
	obs, err := objectsegmentation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = obs.GetObjectPointClouds(context.Background(), "fakeCamera", "", config.AttributeMap{})
	test.That(t, err, test.ShouldNotBeNil)

	// fails since camera cannot generate point clouds (no depth in image)
	r = &inject.Robot{}
	_cam := &simpleSource{}
	cam, err := camera.New(_cam, nil, nil)
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

	obs, err = objectsegmentation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	params := config.AttributeMap{
		"min_points_in_plane":   100,
		"min_points_in_segment": 3,
		"clustering_radius_mm":  5.,
	}
	_, err = obs.GetObjectPointClouds(context.Background(), "fakeCamera", segmentation.RadiusClusteringSegmenter, params)
	test.That(t, err.Error(), test.ShouldContainSubstring, "source has no Projector")
}

func TestGetObjectPointClouds(t *testing.T) {
	cfgService := config.Service{}
	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}
	_cam := &cloudSource{}
	cam, err := camera.New(_cam, nil, nil)
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

	// from a camera that has a PointCloud func -- apply default
	obs, err := objectsegmentation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	cfg := config.AttributeMap{
		"min_points_in_plane":   100,
		"min_points_in_segment": 3,
		"clustering_radius_mm":  5.0,
	}
	segs, err := obs.GetObjectPointClouds(context.Background(), "fakeCamera", segmentation.RadiusClusteringSegmenter, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segs), test.ShouldEqual, 2)

	expectedBoxes := makeExpectedBoxes(t)
	for _, seg := range segs {
		box, err := pointcloud.BoundingBoxFromPointCloud(seg)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box, test.ShouldNotBeNil)
		test.That(t, box.AlmostEqual(expectedBoxes[0]) || box.AlmostEqual(expectedBoxes[1]), test.ShouldBeTrue)
	}
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

	svc, err := objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	result, err := svc.GetObjectPointClouds(context.Background(), "", "", config.AttributeMap{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 2)
	test.That(t, svc1.timesCalled, test.ShouldEqual, 1)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return "not object segmentation", nil
	}

	svc, err = objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected implementation of objectsegmentation.Service")
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return nil, rdkutils.NewResourceNotFoundError(name)
	}

	svc, err = objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	test.That(t, svc, test.ShouldBeNil)
}

type mock struct {
	objectsegmentation.Service
	timesCalled int
}

func (m *mock) GetObjectPointClouds(ctx context.Context,
	cameraName string,
	segmenterName string,
	params config.AttributeMap) ([]*vision.Object, error) {
	m.timesCalled++
	return []*vision.Object{vision.NewEmptyObject(), vision.NewEmptyObject()}, nil
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
	cam, err := camera.New(_cam, nil, nil)
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
	oss, err := objectsegmentation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	osMap := map[resource.Name]interface{}{
		objectsegmentation.Name: oss,
	}
	svc, err := subtype.New(osMap)
	test.That(t, err, test.ShouldBeNil)
	// test the server/client
	resourceSubtype := registry.ResourceSubtypeLookup(objectsegmentation.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	client, err := objectsegmentation.NewClient(context.Background(), "", listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	params := config.AttributeMap{
		"min_points_in_plane":   100,
		"min_points_in_segment": 3,
		"clustering_radius_mm":  5.,
	}
	segs, err := client.GetObjectPointClouds(context.Background(), "fakeCamera", segmentation.RadiusClusteringSegmenter, params)
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
}
