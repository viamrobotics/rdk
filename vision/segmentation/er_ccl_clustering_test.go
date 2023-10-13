//go:build !no_media

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
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

func TestERCCL(t *testing.T) {
	t.Parallel()
	logger := golog.NewTestLogger(t)
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pointcloud.NewFromFile(artifact.MustPath("pointcloud/intel_d435_pointcloud_424.pcd"), logger)
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
		"min_points_in_plane":         2000,
		"max_dist_from_plane_mm":      12.0,
		"min_points_in_segment":       500,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           10,
		"clustering_strictness":       0.00000001,
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
	tempPCD, err := os.CreateTemp(t.TempDir(), "*.pcd")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(tempPCD.Name())
	err = pointcloud.ToPCD(mergedPc, tempPCD, pointcloud.PCDBinary)
	test.That(t, err, test.ShouldBeNil)
}

func BenchmarkERCCL(b *testing.B) {
	params := &transform.PinholeCameraIntrinsics{ // D435 intrinsics for 424x240
		Width:  424,
		Height: 240,
		Fx:     304.1299133300781,
		Fy:     304.2772216796875,
		Ppx:    213.47967529296875,
		Ppy:    124.63351440429688,
	}
	// create the fake camera
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		img, err := rimage.NewImageFromFile(artifact.MustPath("pointcloud/the_color_image_intel_424.jpg"))
		test.That(b, err, test.ShouldBeNil)
		dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("pointcloud/the_depth_image_intel_424.png"))
		test.That(b, err, test.ShouldBeNil)
		return params.RGBDToPointCloud(img, dm)
	}
	var pts []*vision.Object
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
		"min_points_in_plane":         2000,
		"max_dist_from_plane_mm":      12.0,
		"min_points_in_segment":       500,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           10,
		"clustering_strictness":       0.00000001,
	}

	segmenter, err := segmentation.NewERCCLClustering(objConfig)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pts, err = segmenter(context.Background(), injectCamera)
	}
	// to prevent vars from being optimized away
	if pts == nil || err != nil {
		panic("segmenter didn't work")
	}
}
