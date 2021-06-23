package pointcloud

import (
	"testing"

	"go.viam.com/test"

	"github.com/golang/geo/r3"
)

func makeClouds(t *testing.T) []PointCloud {
	// create cloud 0
	cloud0 := New()
	p00 := NewBasicPoint(0, 0, 0)
	test.That(t, cloud0.Set(p00), test.ShouldBeNil)
	p01 := NewBasicPoint(0, 0, 1)
	test.That(t, cloud0.Set(p01), test.ShouldBeNil)
	p02 := NewBasicPoint(0, 1, 0)
	test.That(t, cloud0.Set(p02), test.ShouldBeNil)
	p03 := NewBasicPoint(0, 1, 1)
	test.That(t, cloud0.Set(p03), test.ShouldBeNil)
	// create cloud 1
	cloud1 := New()
	p10 := NewBasicPoint(30, 0, 0)
	test.That(t, cloud1.Set(p10), test.ShouldBeNil)
	p11 := NewBasicPoint(30, 0, 1)
	test.That(t, cloud1.Set(p11), test.ShouldBeNil)
	p12 := NewBasicPoint(30, 1, 0)
	test.That(t, cloud1.Set(p12), test.ShouldBeNil)
	p13 := NewBasicPoint(30, 1, 1)
	test.That(t, cloud1.Set(p13), test.ShouldBeNil)

	return []PointCloud{cloud0, cloud1}
}

func TestCalculateMean(t *testing.T) {
	clouds := makeClouds(t)
	mean0 := CalculateMeanOfPointCloud(clouds[0])
	test.That(t, mean0, test.ShouldResemble, r3.Vector{0, 0.5, 0.5})
	mean1 := CalculateMeanOfPointCloud(clouds[1])
	test.That(t, mean1, test.ShouldResemble, r3.Vector{30, 0.5, 0.5})
}

func TestMergePointsWithColor(t *testing.T) {
	clouds := makeClouds(t)
	mergedCloud, err := MergePointCloudsWithColor(clouds)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mergedCloud.At(0, 0, 0).Color(), test.ShouldResemble, mergedCloud.At(0, 0, 1).Color())
	test.That(t, mergedCloud.At(0, 0, 0).Color(), test.ShouldNotResemble, mergedCloud.At(30, 0, 0).Color())
}
