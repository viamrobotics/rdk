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

func TestVoxelSegmentMeans(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	}

	// Do voxel segmentation
	voxObjConfig := config.AttributeMap{
		"voxel_size":           1.0,
		"lambda":               0.1,
		"min_points_in_plane":  100,
		"min_point_in_segment": 25,
		"clustering_radius_mm": 7.5,
		"weight_threshold":     0.9,
		"angle_threshold":      30,
		"cosine_threshold":     0.1,
		"distance_threshold":   0.1,
	}

	voxSegments, err := segmentation.RadiusClusteringFromVoxels(context.Background(), cam, voxObjConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(voxSegments), test.ShouldBeGreaterThan, 0)
	// get center points
	for _, seg := range voxSegments {
		mean := pc.CalculateMeanOfPointCloud(seg.PointCloud)
		expMean := seg.Center
		test.That(t, mean, test.ShouldResemble, expMean)
	}
}
