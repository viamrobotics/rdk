package obstaclesdistance

import (
	"context"
	"image/color"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
)

func TestObstacleDist(t *testing.T) {
	inp := DistanceDetectorConfig{
		NumQueries: 10,
	}
	ctx := context.Background()
	r := &inject.Robot{}
	cam := &inject.Camera{}

	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return nil, errors.New("no pointcloud")
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("fakeCamera")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		switch n.Name {
		case "fakeCamera":
			return cam, nil
		default:
			return nil, resource.NewNotFoundError(n)
		}
	}
	name := vision.Named("test_odd")
	srv, err := registerObstacleDistanceDetector(ctx, name, &inp, r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, srv.Name(), test.ShouldResemble, name)
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	// Does not implement Detections
	_, err = srv.Detections(ctx, img, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")

	// Does not implement Classifications
	_, err = srv.Classifications(ctx, img, 1, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")

	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		cloud := pc.New()
		err = cloud.Set(pc.NewVector(0, 0, 1), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		return cloud, err
	}
	objects, err := srv.GetObjectPointClouds(ctx, "fakeCamera", nil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(objects), test.ShouldEqual, 1)
	_, isPoint := objects[0].PointCloud.At(0, 0, 1)
	test.That(t, isPoint, test.ShouldBeTrue)

	point := objects[0].Geometry.Pose().Point()
	test.That(t, point.X, test.ShouldEqual, 0)
	test.That(t, point.Y, test.ShouldEqual, 0)
	test.That(t, point.Z, test.ShouldEqual, 1)

	count := 0
	nums := []float64{10, 9, 4, 5, 3, 1, 2, 6, 7, 8}
	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		cloud := pc.New()
		err = cloud.Set(pc.NewVector(0, 0, nums[count]), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		count++
		return cloud, err
	}
	objects, err = srv.GetObjectPointClouds(ctx, "fakeCamera", nil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(objects), test.ShouldEqual, 1)

	_, isPoint = objects[0].PointCloud.At(0, 0, 5.5)
	test.That(t, isPoint, test.ShouldBeTrue)

	// error more than one point in cloud
	count = 0
	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		cloud := pc.New()
		err = cloud.Set(pc.NewVector(0, 0, nums[count]), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(0, 0, 6.0), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		return cloud, err
	}
	_, err = srv.GetObjectPointClouds(ctx, "fakeCamera", nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "obstacles_distance expects one point in the point cloud")

	inp.NumQueries = 0 // value out of range
	_, err = inp.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid number of queries")

	// with error - nil parameters
	_, err = registerObstacleDistanceDetector(ctx, name, nil, r)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot be nil")
}
