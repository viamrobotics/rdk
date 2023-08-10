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
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/rplidar-1_data_2023-06-16T21_42_25.8172Z.pcd", logger)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/pc005522.pcd", logger)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_shoe.pcd", logger)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_wall.pcd", logger)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_misc_items.pcd", logger)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_table_leg.pcd", logger)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_shoe.pcd", nil)
		return pc.NewFromFile(artifact.MustPath("pointcloud/test.las"), logger)
		// return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	}

	objConfig := utils.AttributeMap{
		// lidar config
		// "min_points_in_plane":         1500,
		// "max_dist_from_plane_mm":      100.0,
		// "min_points_in_segment":       150,
		// "ground_angle_tolerance_degs": 20,
		// "ground_plane_normal_vec":     r3.Vector{0, 0, 1},
		// "clustering_radius":           5,
		// "beta":                        2,

		// realsense config
		"min_points_in_plane":         1500,
		"max_dist_from_plane_mm":      10.0,
		"min_points_in_segment":       250,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           5,
		"beta":                        3,
	}

	segmenter, err := segmentation.NewERCCLClustering(objConfig)
	test.That(t, err, test.ShouldBeNil)
	_, err = segmenter(context.Background(), injectCamera)
	test.That(t, err, test.ShouldBeNil)

	// cloudsWithOffset := make([]pc.CloudAndOffsetFunc, 0, len(segments))
	// for _, cloud := range segments {
	// 	cloud := cloud.PointCloud
	// 	cloudCopy := cloud
	// 	cloudFunc := func(ctx context.Context) (pc.PointCloud, spatialmath.Pose, error) {
	// 		return cloudCopy, nil, nil
	// 	}
	// 	cloudsWithOffset = append(cloudsWithOffset, cloudFunc)
	// }
	// mergedCloud, err := pc.MergePointClouds(context.Background(), cloudsWithOffset, logger)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, mergedCloud, test.ShouldNotBeNil)
	// pcdFile, err := os.Create("ERCCL_temp.pcd")
	// test.That(t, err, test.ShouldBeNil)
	// defer pcdFile.Close()
	// pc.ToPCD(mergedCloud, pcdFile, pc.PCDBinary)

	// cloud, _ := pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_shoe.pcd", nil)
	// ps := segmentation.NewPointCloudGroundPlaneSegmentation(cloud, 20, 1500, 20, r3.Vector{0, -1, 0})
	// plane, _, _ := ps.FindGroundPlane(nil)
	// fmt.Println(plane.Equation())
	// planeFile, _ := os.Create("ERCCLshoe_plane.pcd")
	// defer planeFile.Close()
	// planeCloud, _ := plane.PointCloud()
	// pc.ToPCD(planeCloud, planeFile, pc.PCDBinary)
}

func TestSaveGroundPlaneSegmentation(t *testing.T) {
	// cloud, _ := pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/pc005522.pcd", nil)
	// cloud, _ := pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_table_leg.pcd", nil)
	cloud, _ := pc.NewFromFile(artifact.MustPath("pointcloud/test.las"), nil)
	// lidar
	ps := segmentation.NewPointCloudGroundPlaneSegmentation(cloud, 100, 1500, 20, r3.Vector{0, 0, 1})
	// realsense
	// ps := segmentation.NewPointCloudGroundPlaneSegmentation(cloud, 10, 1500, 20, r3.Vector{0, -1, 0})
	plane, _, _ := ps.FindGroundPlane(nil)
	test.That(t, plane, test.ShouldNotBeNil)
	// planeFile, _ := os.Create("pointcloud_ground.pcd")
	// defer planeFile.Close()
	// planeCloud, _ := plane.PointCloud()
	// pc.ToPCD(planeCloud, planeFile, pc.PCDBinary)
}

func BenchmarkERCCL(b *testing.B) {
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), nil)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_wall.pcd", nil)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_misc_items.pcd", nil)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/pc005522.pcd", nil)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_table_leg.pcd", nil)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_shoe.pcd", nil)
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
		// "beta":                        2,

		// realsense config
		"min_points_in_plane":         1500,
		"max_dist_from_plane_mm":      10.0,
		"min_points_in_segment":       250,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           5,
		"beta":                        3,
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
