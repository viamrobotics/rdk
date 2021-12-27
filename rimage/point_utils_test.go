package rimage

import (
	"image"
	"math"
	"testing"

	"github.com/golang/geo/r2"
	"go.viam.com/test"
)

func TestPointDistance(t *testing.T) {
	test.That(t, PointDistance(image.Point{0, 3}, image.Point{4, 0}), test.ShouldEqual, 5.0)
}

func TestPointCenter(t *testing.T) {
	all := []image.Point{
		{0, 0},
		{2, 0},
		{0, 2},
		{2, 2},
	}

	test.That(t, Center(all, 1000), test.ShouldResemble, image.Point{1, 1})

	all = append(all, image.Point{100, 100})

	test.That(t, Center(all, 1000), test.ShouldResemble, image.Point{50, 50})
	test.That(t, Center(all, 48), test.ShouldResemble, image.Point{1, 1})
}

func TestPointAngle(t *testing.T) {
	test.That(t, PointAngle(image.Point{0, 0}, image.Point{1, 0}), test.ShouldEqual, 0.0)
	test.That(t, PointAngle(image.Point{0, 0}, image.Point{1, 1}), test.ShouldEqual, math.Pi/4)
	test.That(t, PointAngle(image.Point{0, 0}, image.Point{-1, -1}), test.ShouldEqual, math.Pi/4-math.Pi)
}

func TestPointBoundingBox(t *testing.T) {
	r := BoundingBox([]image.Point{
		{100, 100},
		{200, 200},
		{50, 50},
		{1000, 1000},
		{1, 1},
	})

	test.That(t, r.Min, test.ShouldResemble, image.Point{1, 1})
	test.That(t, r.Max, test.ShouldResemble, image.Point{1000, 1000})
}

func TestR2ToImage(t *testing.T) {
	imagePoint := image.Point{2, 3}

	test.That(t, R2PointToImagePoint(r2.Point{2.36, 3.004}), test.ShouldResemble, imagePoint)
	test.That(t, R2PointToImagePoint(r2.Point{2.5, 3.5}), test.ShouldNotEqual, imagePoint)

	imageRect := image.Rect(-2, 1, 5, 8)
	r2Rect := r2.RectFromPoints(r2.Point{-2.1, 1.44}, r2.Point{5.33, 8.49})

	test.That(t, R2RectToImageRect(r2Rect), test.ShouldResemble, imageRect)

	r2Rect2 := r2.RectFromPoints(r2.Point{-2.5, 1.5}, r2.Point{5.0, 8.49})

	test.That(t, R2RectToImageRect(r2Rect2), test.ShouldNotEqual, imageRect)
	test.That(t, R2RectToImageRect(r2Rect2), test.ShouldResemble, image.Rect(-3, 2, 5, 8))

	resultImageRect := imageRect.Add(imagePoint)
	resultR2Rect := TranslateR2Rect(r2Rect, r2.Point{2., 3.})

	test.That(t, R2RectToImageRect(resultR2Rect), test.ShouldResemble, resultImageRect)
}
