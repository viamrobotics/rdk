package segmentation_test

import (
	"context"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/pointcloud"
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
		//return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
		return pc.NewFromFile(artifact.MustPath("pointcloud/intel_d435_pointcloud.pcd"), logger)
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
		"min_points_in_plane":         3500,
		"max_dist_from_plane_mm":      10.0,
		"min_points_in_segment":       1000,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           20,
		"clustering_strictness":       2,
	}

	segmenter, err := segmentation.NewERCCLClustering(objConfig)
	test.That(t, err, test.ShouldBeNil)
	objects, err := segmenter(context.Background(), injectCamera)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(objects), test.ShouldEqual, 3)

	pcs := make([]pointcloud.PointCloud, len(objects))
	for i, pc := range objects {
		pcs[i] = pc.PointCloud
	}
	mergedPc, err := pointcloud.MergePointCloudsWithColor(pcs)
	test.That(t, err, test.ShouldBeNil)
	tempPCD, err := os.CreateTemp(".", "*.pcd")
	test.That(t, err, test.ShouldBeNil)
	err = pointcloud.ToPCD(mergedPc, tempPCD, pointcloud.PCDBinary)
	test.That(t, err, test.ShouldBeNil)
}

func BenchmarkERCCL(b *testing.B) {
	logger := golog.NewTestLogger(b)
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		//return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger) // small pc for tests
		return pc.NewFromFile(artifact.MustPath("pointcloud/intel_d435_pointcloud.pcd"), logger)
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
		// "clustering_strictness":      2,

		// realsense config
		"min_points_in_plane":         3500,
		"max_dist_from_plane_mm":      10.0,
		"min_points_in_segment":       2000,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           30,
		"clustering_strictness":       3,
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
