package radiusclustering

import (
	"context"
	"image/color"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
	"go.viam.com/test"
)

func TestRadiusClusteringSegmentation(t *testing.T) {
	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return nil, errors.New("no pointcloud")
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
	params := &segmentation.RadiusClusteringConfig{
		MinPtsInPlane:      100,
		MinPtsInSegment:    3,
		ClusteringRadiusMm: 5.,
		MeanKFiltering:     10.,
	}
	// bad registration, no parameters
	_, err := registerRCSegmenter(context.Background(), "test_rcs", nil, r, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot be nil")
	// bad registration, parameters out of bounds
	params.ClusteringRadiusMm = -3.0
	_, err = registerRCSegmenter(context.Background(), "test_rcs", params, r, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "segmenter config error")
	// successful registration
	params.ClusteringRadiusMm = 5.0
	seg, err := registerRCSegmenter(context.Background(), "test_rcs", params, r, logger)
	test.That(t, err, test.ShouldBeNil)

	// fails on not finding camera
	_, err = seg.GetObjectPointClouds(context.Background(), "no_camera", map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	// fails since camera cannot generate point clouds
	_, err = seg.GetObjectPointClouds(context.Background(), "fakeCamera", map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no pointcloud")

	// successful, creates two clusters of points
	cam.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		cloud := pointcloud.New()
		// cluster 1
		err = cloud.Set(pc.NewVector(1, 1, 1), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(1, 1, 2), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(1, 1, 3), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(1, 1, 4), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		// cluster 2
		err = cloud.Set(pc.NewVector(1, 1, 101), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(1, 1, 102), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(1, 1, 103), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(1, 1, 104), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		return cloud, nil
	}
	objects, err := seg.GetObjectPointClouds(context.Background(), "fakeCamera", map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(objects), test.ShouldEqual, 2)
	// does not implement detector
	_, err = seg.Detections(context.Background(), nil, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")
}
