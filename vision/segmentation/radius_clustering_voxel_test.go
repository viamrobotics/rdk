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
	cfg.RadiusClusteringConfig = &segmentation.RadiusClusteringConfig{}
	cfg.RadiusClusteringConfig.MinPtsInSegment = 5
	cfg.RadiusClusteringConfig.ClusteringRadiusMm = 5
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "min_points_in_plane must be greater than 0")
	// invalid plane config
	cfg.RadiusClusteringConfig.MinPtsInPlane = 5
	cfg.VoxelGridPlaneConfig = &segmentation.VoxelGridPlaneConfig{}
	cfg.VoxelGridPlaneConfig.WeightThresh = -1
	cfg.VoxelGridPlaneConfig.AngleThresh = 40
	cfg.VoxelGridPlaneConfig.CosineThresh = .1
	cfg.VoxelGridPlaneConfig.DistanceThresh = 44
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "weight_threshold cannot be less than 0")
	// valid
	cfg.VoxelGridPlaneConfig.WeightThresh = 1
	err = cfg.CheckValid()
	test.That(t, err, test.ShouldBeNil)
}

func TestVoxelSegmentMeans(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	}

	// Do voxel segmentation
	voxObjConfig := config.AttributeMap{
		"voxel_size":            1.0,
		"lambda":                0.1,
		"min_points_in_plane":   100,
		"min_points_in_segment": 25,
		"clustering_radius_mm":  7.5,
		"weight_threshold":      0.9,
		"angle_threshold":       30,
		"cosine_threshold":      0.1,
		"distance_threshold":    0.1,
	}

	voxSegments, err := segmentation.RadiusClusteringFromVoxels(context.Background(), cam, voxObjConfig)
	test.That(t, err, test.ShouldBeNil)
	testSegmentation(t, voxSegments)
}
