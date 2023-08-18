package segmentation_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

func TestERCCL(t *testing.T) {
	t.Parallel()
	logger := golog.NewTestLogger(t)
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	}

	objConfig := utils.AttributeMap{
		// lidar config
		// "min_points_in_plane":         1500,
		// "max_dist_from_plane_mm":      100.0,
		// "min_points_in_segment":       150,
		// "ground_angle_tolerance_degs": 20,
		// "ground_plane_normal_vec":     r3.Vector{0, 0, 1},
		// "clustering_radius":           5,
		// "clustering_granularity":      2,

		// realsense config
		"min_points_in_plane":         1500,
		"max_dist_from_plane_mm":      10.0,
		"min_points_in_segment":       250,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           5,
		"clustering_granularity":      3,
	}

	segmenter, err := segmentation.NewERCCLClustering(objConfig)
	test.That(t, err, test.ShouldBeNil)
	_, err = segmenter(context.Background(), injectCamera)
	test.That(t, err, test.ShouldBeNil)
}

func BenchmarkERCCL(b *testing.B) {
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), nil)
	}
	var pts []*vision.Object
	var err error
	// do segmentation
	objConfig := utils.AttributeMap{
		// lidar config
		// "min_points_in_plane":         1500,
		// "max_dist_from_plane_mm":      100.0,
		// "min_points_in_segment":       150,
		// "ground_angle_tolerance_degs": 20,
		// "ground_plane_normal_vec":     r3.Vector{0, 0, 1},
		// "clustering_radius":           5,
		// "clustering_granularity":      2,

		// realsense config
		"min_points_in_plane":         1500,
		"max_dist_from_plane_mm":      10.0,
		"min_points_in_segment":       250,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           5,
		"clustering_granularity":      3,
	}

	segmenter, err := segmentation.NewERCCLClustering(objConfig)

	for i := 0; i < b.N; i++ {
		pts, err = segmenter(context.Background(), injectCamera)
	}
	// to prevent vars from being optimized away
	if pts == nil || err != nil {
		panic("segmenter didn't work")
	}
}
