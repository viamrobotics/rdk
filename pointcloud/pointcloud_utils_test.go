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
	p00 := NewVector(0, 0, 0)
	test.That(t, cloud0.Set(p00, nil), test.ShouldBeNil)
	p01 := NewVector(0, 0, 1)
	test.That(t, cloud0.Set(p01, nil), test.ShouldBeNil)
	p02 := NewVector(0, 1, 0)
	test.That(t, cloud0.Set(p02, nil), test.ShouldBeNil)
	p03 := NewVector(0, 1, 1)
	test.That(t, cloud0.Set(p03, nil), test.ShouldBeNil)
	// create cloud 1
	cloud1 := New()
	p10 := NewVector(30, 0, 0)
	test.That(t, cloud1.Set(p10, nil), test.ShouldBeNil)
	p11 := NewVector(30, 0, 1)
	test.That(t, cloud1.Set(p11, nil), test.ShouldBeNil)
	p12 := NewVector(30, 1, 0)
	test.That(t, cloud1.Set(p12, nil), test.ShouldBeNil)
	p13 := NewVector(30, 1, 1)
	test.That(t, cloud1.Set(p13, nil), test.ShouldBeNil)
	p14 := NewVector(28, 0.5, 0.5)
	test.That(t, cloud1.Set(p14, nil), test.ShouldBeNil)

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
		expectedBox, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(c.expectedCenter), c.expectedDims, "")
		test.That(t, err, test.ShouldBeNil)
		box, err := BoundingBoxFromPointCloudWithLabel(c.pc, "box")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box, test.ShouldNotBeNil)
		test.That(t, spatialmath.GeometriesAlmostEqual(box, expectedBox), test.ShouldBeTrue)
		test.That(t, box.Label(), test.ShouldEqual, "box")
	}
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

func TestToOctree(t *testing.T) {
	pc := newBigPC()
	tree, err := ToBasicOctree(pc)
	test.That(t, err, test.ShouldBeNil)
	pc.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		treeData, b := tree.At(p.X, p.Y, p.Z)
		test.That(t, b, test.ShouldBeTrue)
		test.ShouldResemble(t, treeData, d)
		return true
	})

	basicTree, err := createNewOctree(r3.Vector{0, 0, 0}, 2)
	test.That(t, err, test.ShouldBeNil)
	pointsAndData := []PointAndData{
		{P: r3.Vector{X: 0, Y: 0, Z: 0}, D: NewValueData(2)},
		{P: r3.Vector{X: .5, Y: 0, Z: 0}, D: NewValueData(3)},
		{P: r3.Vector{X: .5, Y: 0, Z: .5}, D: NewValueData(10)},
		{P: r3.Vector{X: .5, Y: .5, Z: 0}, D: NewValueData(1)},
		{P: r3.Vector{X: .55, Y: .55, Z: 0}, D: NewValueData(4)},
		{P: r3.Vector{X: -.55, Y: -.55, Z: 0}, D: NewValueData(5)},
		{P: r3.Vector{X: .755, Y: .755, Z: 0}, D: NewValueData(6)},
	}

	err = addPoints(basicTree, pointsAndData)
	test.That(t, err, test.ShouldBeNil)
	tree, err = ToBasicOctree(basicTree)
	test.That(t, err, test.ShouldBeNil)
	basicTree.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		treeData, b := tree.At(p.X, p.Y, p.Z)
		test.That(t, b, test.ShouldBeTrue)
		test.ShouldResemble(t, treeData, d)
		return true
	})
}
