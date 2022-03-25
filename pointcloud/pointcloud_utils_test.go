package pointcloud

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

func makeClouds(t *testing.T) []PointCloud {
	t.Helper()
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
	p14 := NewBasicPoint(28, 0.5, 0.5)
	test.That(t, cloud1.Set(p14), test.ShouldBeNil)

	return []PointCloud{cloud0, cloud1}
}

func TestBoundingBoxFromPointCloud(t *testing.T) {
	clouds := makeClouds(t)
	cases := []struct {
		pc             PointCloud
		expectedCenter r3.Vector
		expectedDims   r3.Vector
	}{
		{clouds[0], r3.Vector{0, 0.5, 0.5}, r3.Vector{0, 1, 1}},
		{clouds[1], r3.Vector{29.6, 0.5, 0.5}, r3.Vector{2, 1, 1}},
	}

	for _, c := range cases {
		expectedBox, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(c.expectedCenter), c.expectedDims)
		test.That(t, err, test.ShouldBeNil)
		box, err := BoundingBoxFromPointCloud(c.pc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box, test.ShouldNotBeNil)
		test.That(t, box.AlmostEqual(expectedBox), test.ShouldBeTrue)
	}
}

func TestMergePoints(t *testing.T) {
	clouds := makeClouds(t)
	mergedCloud, err := MergePointClouds(clouds)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mergedCloud.At(0, 0, 0), test.ShouldNotBeNil)
	test.That(t, mergedCloud.At(30, 0, 0), test.ShouldNotBeNil)
}

func TestMergePointsWithColor(t *testing.T) {
	clouds := makeClouds(t)
	mergedCloud, err := MergePointCloudsWithColor(clouds)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mergedCloud.At(0, 0, 0).Color(), test.ShouldResemble, mergedCloud.At(0, 0, 1).Color())
	test.That(t, mergedCloud.At(0, 0, 0).Color(), test.ShouldNotResemble, mergedCloud.At(30, 0, 0).Color())
}

func TestPrune(t *testing.T) {
	clouds := makeClouds(t)
	// before prune
	test.That(t, len(clouds), test.ShouldEqual, 2)
	test.That(t, clouds[0].Size(), test.ShouldEqual, 4)
	test.That(t, clouds[1].Size(), test.ShouldEqual, 5)
	// prune
	clouds = PrunePointClouds(clouds, 5)
	test.That(t, len(clouds), test.ShouldEqual, 1)
	test.That(t, clouds[0].Size(), test.ShouldEqual, 5)
}
