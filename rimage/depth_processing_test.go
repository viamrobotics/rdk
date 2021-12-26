package rimage

import (
	"testing"

	"github.com/golang/geo/r2"
	"go.viam.com/test"
)

type rangeArrayHelper struct {
	dim      int
	expected []int
}

func TestRangeArray(t *testing.T) {
	cases := []rangeArrayHelper{
		{5, []int{-2, -1, 0, 1, 2}},
		{4, []int{-2, -1, 0, 1}},
		{3, []int{-1, 0, 1}},
		{2, []int{-1, 0}},
		{1, []int{0}},
		{0, []int{}},
		{-2, []int{}},
	}
	for _, c := range cases {
		got := makeRangeArray(c.dim)
		test.That(t, c.expected, test.ShouldResemble, got)
	}
}

func TestStructuringElement(t *testing.T) {
	expected := &DepthMap{3, 3, []Depth{0, 1, 0, 1, 1, 1, 0, 1, 0}}
	got := makeStructuringElement(3)
	test.That(t, expected, test.ShouldResemble, got)
}

func TestInterpolations(t *testing.T) {
	dm := NewEmptyDepthMap(2, 2)
	dm.Set(0, 0, 1)
	dm.Set(1, 0, 2)
	dm.Set(0, 1, 3)
	dm.Set(1, 1, 4)

	pt := r2.Point{0.25, 0.25}
	d := NearestNeighborDepth(pt, dm)
	test.That(t, *d, test.ShouldEqual, Depth(1))
	d = BilinearInterpolationDepth(pt, dm) // 1.75
	test.That(t, *d, test.ShouldEqual, Depth(2))

	pt = r2.Point{0.25, 0.75}
	d = NearestNeighborDepth(pt, dm)
	test.That(t, *d, test.ShouldEqual, Depth(3))
	d = BilinearInterpolationDepth(pt, dm) // 2.75
	test.That(t, *d, test.ShouldEqual, Depth(3))

	pt = r2.Point{0.5, 0.5}
	d = NearestNeighborDepth(pt, dm)
	test.That(t, *d, test.ShouldEqual, Depth(4))
	d = BilinearInterpolationDepth(pt, dm) // 2.5
	test.That(t, *d, test.ShouldEqual, Depth(3))

	pt = r2.Point{1.0, 1.0}
	d = NearestNeighborDepth(pt, dm)
	test.That(t, *d, test.ShouldEqual, Depth(4))
	d = BilinearInterpolationDepth(pt, dm) // 4
	test.That(t, *d, test.ShouldEqual, Depth(4))

	pt = r2.Point{1.1, 1.0}
	d = NearestNeighborDepth(pt, dm)
	test.That(t, d, test.ShouldBeNil)
	d = BilinearInterpolationDepth(pt, dm)
	test.That(t, d, test.ShouldBeNil)
}
