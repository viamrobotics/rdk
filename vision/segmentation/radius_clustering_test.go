package segmentation_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/config"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

func TestRadiusClusteringValidate(t *testing.T) {
	cfg := segmentation.RadiusClusteringConfig{}
	// invalid points in plane
	err := cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "min_points_in_plane must be greater than 0")
	// invalid points in segment
	cfg.MinPtsInPlane = 5
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "min_points_in_segment must be greater than 0")
	// invalid clustering radius
	cfg.MinPtsInSegment = 5
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "clustering_radius_mm must be greater than 0")
	// valid
	cfg.ClusteringRadiusMm = 5
	cfg.MeanKFiltering = 5
	err = cfg.CheckValid()
	test.That(t, err, test.ShouldBeNil)
}

// get a segmentation of a pointcloud and calculate each object's center.
func TestPixelSegmentation(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	}
	// do segmentation
	objConfig := config.AttributeMap{
		"min_points_in_plane":   50000,
		"min_points_in_segment": 500,
		"clustering_radius_mm":  10.0,
		"mean_k_filtering":      50.0,
		"extra_uneeded_param":   4444,
		"another_extra_one":     "hey",
	}
	segments, err := segmentation.RadiusClustering(context.Background(), injectCamera, objConfig)
	test.That(t, err, test.ShouldBeNil)
	testSegmentation(t, segments)
	// do segmentation with no mean k filtering
	objConfig = config.AttributeMap{
		"min_points_in_plane":   50000,
		"min_points_in_segment": 500,
		"clustering_radius_mm":  10.0,
		"mean_k_filtering":      -1.,
		"extra_uneeded_param":   4444,
		"another_extra_one":     "hey",
	}
	segments, err = segmentation.RadiusClustering(context.Background(), injectCamera, objConfig)
	test.That(t, err, test.ShouldBeNil)
	testSegmentation(t, segments)
}

func testSegmentation(t *testing.T, segments []*vision.Object) {
	t.Helper()
	test.That(t, len(segments), test.ShouldBeGreaterThan, 0)
	for _, seg := range segments {
		box, err := pc.BoundingBoxFromPointCloud(seg)
		if seg.Size() == 0 {
			test.That(t, box, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			continue
		}
		test.That(t, box, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box.AlmostEqual(seg.BoundingBox), test.ShouldBeTrue)
	}
}
