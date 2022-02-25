package segmentation

import (
	"context"
	"testing"

	"go.viam.com/test"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/vision"
)

func TestVoxelSegmentation(t *testing.T) {
	cloud := loadPointCloud(t)

	// turn pointclouds into voxel grid
	vg := pc.NewVoxelGridFromPointCloud(cloud, 1.0, 0.1)

	// Do voxel segmentation
	voxPlaneConfig := VoxelGridPlaneConfig{
		weightThresh:   0.9,
		angleThresh:    30,
		cosineThresh:   0.1,
		distanceThresh: 0.1,
	}
	voxObjConfig := &vision.Parameters3D{
		MinPtsInPlane:      100,
		MinPtsInSegment:    25,
		ClusteringRadiusMm: 7.5,
	}

	segmentation, err := NewObjectSegmentationFromVoxelGrid(context.Background(), vg, voxObjConfig, voxPlaneConfig)
	test.That(t, err, test.ShouldBeNil)
	testSegmentation(t, segmentation)
}
