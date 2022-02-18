package objectsegmentation_test

import (
	"context"
	"image"
	"net"
	"testing"

	"github.com/edaniels/golog"
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
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

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
	err := pcA.Set(pointcloud.NewBasicPoint(5, 5, 5))
	if err != nil {
		return nil, err
	}
	err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 6))
	if err != nil {
		return nil, err
	}
	err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 4))
	if err != nil {
		return nil, err
	}
	err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 5))
	if err != nil {
		return nil, err
	}
	err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 6))
	if err != nil {
		return nil, err
	}
	err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 4))
	if err != nil {
		return nil, err
	}
	return pcA, nil
}

func TestServiceFailures(t *testing.T) {
	cfgService := config.Service{}
	logger := golog.NewTestLogger(t)

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(resource.Name) (interface{}, bool) {
		return nil, false
	}
	// fails on not finding the service
	_, err := objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rdkutils.NewResourceNotFoundError(objectsegmentation.Name))

	// fails on not finding camera
	obs, err := objectsegmentation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = obs.GetObjectPointClouds(context.Background(), "fakeCamera", &vision.Parameters3D{})
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
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, bool) {
		switch n.Name {
		case "fakeCamera":
			return cam, true
		default:
			return nil, false
		}
	}

	obs, err = objectsegmentation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = obs.GetObjectPointClouds(context.Background(), "fakeCamera", &vision.Parameters3D{})
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
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, bool) {
		switch n.Name {
		case "fakeCamera":
			return cam, true
		default:
			return nil, false
		}
	}

	// from a camera that has a PointCloud func -- apply default
	obs, err := objectsegmentation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	segs, err := obs.GetObjectPointClouds(context.Background(), "fakeCamera", &vision.Parameters3D{100, 3, 5.})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segs), test.ShouldEqual, 2)
	test.That(t, segs[0].Center.Z, test.ShouldEqual, 5.)
	test.That(t, segs[1].Center.Z, test.ShouldEqual, 5.)
	test.That(t, segs[0].BoundingBox.WidthMm, test.ShouldEqual, 0)
	test.That(t, segs[0].BoundingBox.LengthMm, test.ShouldEqual, 0)
	test.That(t, segs[0].BoundingBox.DepthMm, test.ShouldEqual, 2)
	test.That(t, segs[1].BoundingBox.WidthMm, test.ShouldEqual, 0)
	test.That(t, segs[1].BoundingBox.LengthMm, test.ShouldEqual, 0)
	test.That(t, segs[1].BoundingBox.DepthMm, test.ShouldEqual, 2)
}

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return svc1, true
	}
	return r, svc1
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	result, err := svc.GetObjectPointClouds(context.Background(), "", &vision.Parameters3D{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 2)
	test.That(t, svc1.timesCalled, test.ShouldEqual, 1)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return "not object segmentation", true
	}

	svc, err = objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected implementation of objectsegmentation.Service")
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return nil, false
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

func (m *mock) GetObjectPointClouds(ctx context.Context, cameraName string, params *vision.Parameters3D) ([]*vision.Object, error) {
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
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, bool) {
		switch n.Name {
		case "fakeCamera":
			return cam, true
		default:
			return nil, false
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
	segs, err := client.GetObjectPointClouds(context.Background(), "fakeCamera", &vision.Parameters3D{100, 3, 5.})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segs), test.ShouldEqual, 2)
	test.That(t, segs[0].Center.Z, test.ShouldEqual, 5.)
	test.That(t, segs[1].Center.Z, test.ShouldEqual, 5.)
	test.That(t, segs[0].BoundingBox.WidthMm, test.ShouldEqual, 0)
	test.That(t, segs[0].BoundingBox.LengthMm, test.ShouldEqual, 0)
	test.That(t, segs[0].BoundingBox.DepthMm, test.ShouldEqual, 2)
	test.That(t, segs[1].BoundingBox.WidthMm, test.ShouldEqual, 0)
	test.That(t, segs[1].BoundingBox.LengthMm, test.ShouldEqual, 0)
	test.That(t, segs[1].BoundingBox.DepthMm, test.ShouldEqual, 2)

	test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
}
