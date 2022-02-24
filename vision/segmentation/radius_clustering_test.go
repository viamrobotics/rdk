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
	"go.viam.com/rdk/vision/segmentation"
)

// get a segmentation of a pointcloud and calculate each object's center.
func TestCalculateSegmentMeans(t *testing.T) {
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
		"extra_uneeded_param":   4444,
		"another_extra_one":     "hey",
	}
	segments, err := segmentation.RadiusClustering(context.Background(), injectCamera, objConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segments), test.ShouldBeGreaterThan, 0)
	// get center points
	for _, seg := range segments {
		mean := pc.CalculateMeanOfPointCloud(seg.PointCloud)
		expMean := seg.Center
		test.That(t, mean, test.ShouldResemble, expMean)
	}
}
