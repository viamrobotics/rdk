package rimage

import (
	"image"
	"testing"

	"github.com/golang/geo/r2"
	"go.viam.com/test"
)

func TestBorderType(t *testing.T) {
	test.That(t, Hole, test.ShouldEqual, 1)
	test.That(t, Outer, test.ShouldEqual, 2)
}

func TestCreateHoleBorder(t *testing.T) {
	border := CreateHoleBorder()
	test.That(t, border.borderType, test.ShouldEqual, Hole)
}

func TestContourFloat(t *testing.T) {
	contour0 := ContourFloat{}
	test.That(t, len(contour0), test.ShouldEqual, 0)
	contour1 := ContourFloat{r2.Point{X: 0.5, Y: 1.5}}
	test.That(t, len(contour1), test.ShouldEqual, 1)
}

func TestContourImagePoint(t *testing.T) {
	contour0 := ContourInt{}
	test.That(t, len(contour0), test.ShouldEqual, 0)
	contour1 := ContourInt{image.Point{X: 0, Y: 1}}
	test.That(t, len(contour1), test.ShouldEqual, 1)
}

func TestContourPoint(t *testing.T) {
	contourPoint0 := ContourPoint{}
	test.That(t, contourPoint0.Point.X, test.ShouldEqual, 0)
	test.That(t, contourPoint0.Point.Y, test.ShouldEqual, 0)
	test.That(t, contourPoint0.Idx, test.ShouldEqual, 0)

	pt := r2.Point{18., 22.}
	idx := 5
	contourPoint1 := ContourPoint{pt, idx}
	test.That(t, contourPoint1.Point.X, test.ShouldEqual, 18)
	test.That(t, contourPoint1.Point.Y, test.ShouldEqual, 22)
	test.That(t, contourPoint1.Idx, test.ShouldEqual, 5)
}

func TestGetPairOfFarthestPointsContour(t *testing.T) {
	contour := ContourFloat{{0, 0}, {1, 0}, {1, 1}}
	p0, p1 := GetPairOfFarthestPointsContour(contour)
	test.That(t, p0.Point.X, test.ShouldEqual, 0)
	test.That(t, p0.Point.Y, test.ShouldEqual, 0)
	test.That(t, p1.Point.X, test.ShouldEqual, 1)
	test.That(t, p1.Point.Y, test.ShouldEqual, 1)
}

func TestIsContourClosed(t *testing.T) {
	contour := ContourFloat{{0, 0}, {1, 0}, {1, 1}}
	isClosed := IsContourClosed(contour, 2.0)
	test.That(t, isClosed, test.ShouldBeTrue)

	contour1 := ContourFloat{{0, 0}, {0, 1}, {0, 2}, {0, 3}}
	isClosed1 := IsContourClosed(contour1, 1.0)
	test.That(t, isClosed1, test.ShouldBeFalse)
}

func TestSortPointCounterClockwise(t *testing.T) {
	pts := []r2.Point{{0, 0}, {0, 1}, {1, 0}, {1, 1}}
	ptsSorted := SortPointCounterClockwise(pts)
	test.That(t, len(ptsSorted), test.ShouldEqual, 4)
	test.That(t, ptsSorted[0], test.ShouldResemble, r2.Point{0, 0})
	test.That(t, ptsSorted[1], test.ShouldResemble, r2.Point{1, 0})
	test.That(t, ptsSorted[2], test.ShouldResemble, r2.Point{1, 1})
	test.That(t, ptsSorted[3], test.ShouldResemble, r2.Point{0, 1})

	pts2 := []r2.Point{{0, -20}, {15, -15}, {-15, -15}, {-20, 0}, {20, 0}}
	ptsSorted2 := SortPointCounterClockwise(pts2)
	test.That(t, len(ptsSorted2), test.ShouldEqual, 5)
	test.That(t, ptsSorted2[0], test.ShouldResemble, r2.Point{-15, -15})
	test.That(t, ptsSorted2[1], test.ShouldResemble, r2.Point{0, -20})
	test.That(t, ptsSorted2[2], test.ShouldResemble, r2.Point{15, -15})
	test.That(t, ptsSorted2[3], test.ShouldResemble, r2.Point{20, 0})
	test.That(t, ptsSorted2[4], test.ShouldResemble, r2.Point{-20, 0})
}
