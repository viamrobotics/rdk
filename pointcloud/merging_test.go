package pointcloud

import (
	"image/color"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func makeThreeCloudsWithOffsets(t *testing) []PointCloudWithOffset {
	pc1 := pointcloud.NewWithPrealloc(1)
	err := pc1.Set(pointcloud.NewVector(1, 0, 0), pointcloud.NewColoredData(color.NRGBA{255, 0, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	pc2 := pointcloud.NewWithPrealloc(1)
	err := pc2.Set(pointcloud.NewVector(0, 1, 0), pointcloud.NewColoredData(color.NRGBA{0, 255, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	pc3 := pointcloud.NewWithPrealloc(1)
	err := pc3.Set(pointcloud.NewVector(0, 0, 1), pointcloud.NewColoredData(color.NRGBA{0, 0, 255, 255}))
	test.That(t, err, test.ShouldBeNil)
	pose1 := spatialmath.NewPoseFromPoint(r3.Vector{100, 0, 0})
	pose2 := spatialmath.NewPoseFromPoint(r3.Vector{100, 0, 100})
	pose3 := spatialmath.NewPoseFromPoint(r3.Vector{100, 100, 100})
	return []PointCloudWithOffset{{pc1, pose1}, {pc2, pose2}, {pc3, pose3}}
}

func TestMergePoints1(t *testing.T) {
	clouds := makeClouds(t)
	cloudsWithOffset := make([]PointCloudWithOffset, 0, len(clouds))
	for _, cloud := range clouds {
		cloudCopy := cloud
		cloudsWithOffset = append(cloudsWithOffset, PointCloudWithOffset{cloudCopy, nil})
	}
	mergedCloud, err := MergePointClouds(cloudsWithOffset)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, CloudContains(mergedCloud, 0, 0, 0), test.ShouldBeTrue)
	test.That(t, CloudContains(mergedCloud, 30, 0, 0), test.ShouldBeTrue)
}

func TestMergePointsWithColor(t *testing.T) {
	clouds := makeClouds(t)
	mergedCloud, err := MergePointCloudsWithColor(clouds)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mergedCloud.Size(), test.ShouldResemble, 9)

	a, got := mergedCloud.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	b, got := mergedCloud.At(0, 0, 1)
	test.That(t, got, test.ShouldBeTrue)

	c, got := mergedCloud.At(30, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	test.That(t, a.Color(), test.ShouldResemble, b.Color())
	test.That(t, a.Color(), test.ShouldNotResemble, c.Color())
}
