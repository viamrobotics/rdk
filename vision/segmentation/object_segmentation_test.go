package segmentation

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/vision"
)

// get a segmentation of a pointcloud and calculate each object's center.
func TestCalculateSegmentMeans(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cloud, err := pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	test.That(t, err, test.ShouldBeNil)
	// do segmentation
	objConfig := vision.Parameters3D{
		MinPtsInPlane:      50000,
		MinPtsInSegment:    500,
		ClusteringRadiusMm: 10.0,
	}
	segments, err := NewObjectSegmentation(context.Background(), cloud, objConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segments.N(), test.ShouldBeGreaterThan, 0)
	// get center points
	for i := 0; i < segments.N(); i++ {
		mean := pc.CalculateMeanOfPointCloud(segments.Segments.Objects[i].PointCloud)
		expMean := segments.Segments.Objects[i].Center
		test.That(t, mean, test.ShouldResemble, expMean)
	}
}

func TestVoxelSegmentMeans(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cloud, err := pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	test.That(t, err, test.ShouldBeNil)
	// turn pointclouds into voxel grid
	vg := pc.NewVoxelGridFromPointCloud(cloud, 1.0, 0.1)

	// Do voxel segmentation
	voxPlaneConfig := VoxelGridPlaneConfig{
		weightThresh:   0.9,
		angleThresh:    30,
		cosineThresh:   0.1,
		distanceThresh: 0.1,
	}
	voxObjConfig := vision.Parameters3D{
		MinPtsInPlane:      100,
		MinPtsInSegment:    25,
		ClusteringRadiusMm: 7.5,
	}

	voxSegments, err := NewObjectSegmentationFromVoxelGrid(context.Background(), vg, voxObjConfig, voxPlaneConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, voxSegments.N(), test.ShouldBeGreaterThan, 0)
	// get center points
	for i := 0; i < voxSegments.N(); i++ {
		mean := pc.CalculateMeanOfPointCloud(voxSegments.Segments.Objects[i].PointCloud)
		expMean := voxSegments.Segments.Objects[i].Center
		test.That(t, mean, test.ShouldResemble, expMean)
	}
}
