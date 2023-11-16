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
	"go.viam.com/rdk/vision/segmentation"
)

func TestClusteringVoxelConfig(t *testing.T) {
	// invalid voxel size
	cfg := segmentation.RadiusClusteringVoxelConfig{}
	err := cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "voxel_size must be greater than 0")
	// invalid lambda
	cfg.VoxelSize = 2.0
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "lambda must be greater than 0")
	// invalid clustering
	cfg.Lambda = 0.1
	cfg.MinPtsInSegment = 5
	cfg.ClusteringRadiusMm = 5
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "min_points_in_plane must be greater than 0")
	// invalid plane config
	cfg.MinPtsInPlane = 5
	cfg.WeightThresh = -1
	cfg.AngleThresh = 40
	cfg.CosineThresh = .1
	cfg.DistanceThresh = 44
	cfg.MaxDistFromPlane = 10
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "weight_threshold cannot be less than 0")
	// valid
	cfg.WeightThresh = 1
	err = cfg.CheckValid()
	test.That(t, err, test.ShouldBeNil)
}

func TestVoxelSegmentMeans(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	}

	// Do voxel segmentation
	expectedLabel := "test_label"
	voxObjConfig := utils.AttributeMap{
		"voxel_size":            1.0,
		"lambda":                0.1,
		"min_points_in_plane":   100,
		"max_dist_from_plane":   10,
		"min_points_in_segment": 25,
		"clustering_radius_mm":  7.5,
		"weight_threshold":      0.9,
		"angle_threshold":       30,
		"cosine_threshold":      0.1,
		"distance_threshold":    0.1,
		"label":                 expectedLabel,
	}

	segmenter, err := segmentation.NewRadiusClusteringFromVoxels(voxObjConfig)
	test.That(t, err, test.ShouldBeNil)
	voxSegments, err := segmenter(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	testSegmentation(t, voxSegments, expectedLabel)
}
