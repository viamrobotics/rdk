package segmentation_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
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
	// invalid angle from plane
	cfg.ClusteringRadiusMm = 5
	cfg.AngleTolerance = 190
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "max_angle_of_plane must between 0 & 180 (inclusive)")
	// valid
	cfg.AngleTolerance = 180
	cfg.MeanKFiltering = 5
	cfg.MaxDistFromPlane = 4
	err = cfg.CheckValid()
	test.That(t, err, test.ShouldBeNil)

	// cfg succeeds even without MaxDistFromPlane, AngleTolerance, NormalVec
	cfg = segmentation.RadiusClusteringConfig{
		MinPtsInPlane:      10,
		MinPtsInSegment:    10,
		ClusteringRadiusMm: 10,
		MeanKFiltering:     10,
	}
	err = cfg.CheckValid()
	test.That(t, err, test.ShouldBeNil)
}

// get a segmentation of a pointcloud and calculate each object's center.
func TestPixelSegmentation(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	}
	// do segmentation
	expectedLabel := "test_label"
	objConfig := utils.AttributeMap{
		"min_points_in_plane":   50000,
		"max_dist_from_plane":   10,
		"min_points_in_segment": 500,
		"clustering_radius_mm":  10.0,
		"mean_k_filtering":      50.0,
		"extra_uneeded_param":   4444,
		"another_extra_one":     "hey",
		"label":                 expectedLabel,
	}
	segmenter, err := segmentation.NewRadiusClustering(objConfig)
	test.That(t, err, test.ShouldBeNil)
	segments, err := segmenter(context.Background(), injectCamera)
	test.That(t, err, test.ShouldBeNil)
	testSegmentation(t, segments, expectedLabel)
}

func TestPixelSegmentationNoFiltering(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	}
	// do segmentation with no mean k filtering
	expectedLabel := "test_label"
	objConfig := utils.AttributeMap{
		"min_points_in_plane":   9000,
		"max_dist_from_plane":   110.0,
		"min_points_in_segment": 350,
		"max_angle_of_plane":    30,
		"clustering_radius_mm":  600.0,
		"mean_k_filtering":      0,
		"extra_uneeded_param":   4444,
		"another_extra_one":     "hey",
		"label":                 expectedLabel,
	}
	segmenter, err := segmentation.NewRadiusClustering(objConfig)
	test.That(t, err, test.ShouldBeNil)
	segments, err := segmenter(context.Background(), injectCamera)
	test.That(t, err, test.ShouldBeNil)
	testSegmentation(t, segments, expectedLabel)
}

func testSegmentation(t *testing.T, segments []*vision.Object, expectedLabel string) {
	t.Helper()
	test.That(t, len(segments), test.ShouldBeGreaterThan, 0)
	for _, seg := range segments {
		box, err := pc.BoundingBoxFromPointCloudWithLabel(seg, seg.Geometry.Label())
		if seg.Size() == 0 {
			test.That(t, box, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			continue
		}
		test.That(t, box, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box.AlmostEqual(seg.Geometry), test.ShouldBeTrue)
		test.That(t, box.Label(), test.ShouldEqual, expectedLabel)
	}
}

func BenchmarkRadiusClustering(b *testing.B) {
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), nil)
	}
	var pts []*vision.Object
	var err error
	// do segmentation
	objConfig := utils.AttributeMap{
		"min_points_in_plane":   9000,
		"max_dist_from_plane":   110.0,
		"min_points_in_segment": 350,
		"max_angle_of_plane":    30,
		"clustering_radius_mm":  600.0,
		"mean_k_filtering":      0,
	}
	segmenter, _ := segmentation.NewRadiusClustering(objConfig)
	for i := 0; i < b.N; i++ {
		pts, err = segmenter(context.Background(), injectCamera)
	}
	// to prevent vars from being optimized away
	if pts == nil || err != nil {
		panic("segmenter didn't work")
	}
}
