package segmentation_test

import (
	"context"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
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
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/pointcloud.pcd", logger)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_shoe.pcd", logger)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_wall.pcd", logger)
		return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_misc_items.pcd", logger)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_table_leg.pcd", logger)
		// return pc.NewFromFile(artifact.MustPath("pointcloud/test.las"), logger)
		// return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	}

	objConfig := utils.AttributeMap{
		"min_points_in_plane":         1500,
		"max_dist_from_plane_mm":      10.0,
		"min_points_in_segment":       50,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           5,
		"s":                           20,
		// "alpha":                       0.9,
		// "beta":                        0.5,
	}

	segmenter, err := segmentation.NewERCCLClustering(objConfig)
	test.That(t, err, test.ShouldBeNil)
	segments, err := segmenter(context.Background(), injectCamera)
	test.That(t, err, test.ShouldBeNil)

	cloudsWithOffset := make([]pc.CloudAndOffsetFunc, 0, len(segments))
	for _, cloud := range segments {
		cloud := cloud.PointCloud
		cloudCopy := cloud
		cloudFunc := func(ctx context.Context) (pc.PointCloud, spatialmath.Pose, error) {
			return cloudCopy, nil, nil
		}
		cloudsWithOffset = append(cloudsWithOffset, cloudFunc)
	}
	mergedCloud, err := pc.MergePointClouds(context.Background(), cloudsWithOffset, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mergedCloud, test.ShouldNotBeNil)
	pcdFile, err := os.Create("ERCCLmisc.pcd")
	test.That(t, err, test.ShouldBeNil)
	defer pcdFile.Close()
	pc.ToPCD(mergedCloud, pcdFile, pc.PCDBinary)

	// cloud, _ := pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_shoe.pcd", nil)
	// ps := segmentation.NewPointCloudGroundPlaneSegmentation(cloud, 20, 1500, 20, r3.Vector{0, -1, 0})
	// plane, _, _ := ps.FindGroundPlane(nil)
	// fmt.Println(plane.Equation())
	// planeFile, _ := os.Create("ERCCLshoe_plane.pcd")
	// defer planeFile.Close()
	// planeCloud, _ := plane.PointCloud()
	// pc.ToPCD(planeCloud, planeFile, pc.PCDBinary)
}

func BenchmarkERCCL(b *testing.B) {
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		// return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), nil)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_wall.pcd", nil)
		return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_misc_items.pcd", nil)
		// return pc.NewFromFile("/Users/vpandiarajan/viam/robotManip/pythonManip/realsense_shoe.pcd", nil)
	}
	var pts []*vision.Object
	var err error
	// do segmentation
	objConfig := utils.AttributeMap{
		"min_points_in_plane":         1500,
		"max_dist_from_plane_mm":      10.0,
		"min_points_in_segment":       50,
		"ground_angle_tolerance_degs": 20,
		"ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		"clustering_radius":           5,
		"s":                           20,
		// "min_points_in_plane":         900,
		// "max_dist_from_plane_mm":      60.0,
		// "min_points_in_segment":       50,
		// "ground_angle_tolerance_degs": 30,
		// "ground_plane_normal_vec":     r3.Vector{0, -1, 0},
		// "radius":                      5,
		// "alpha":                       0.5,
		// "beta":                        0.5,
		// "s":                           200,
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
