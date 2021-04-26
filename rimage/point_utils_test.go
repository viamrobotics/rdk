package rimage

import (
	"image"
	"math"
	"testing"

	"github.com/golang/geo/r2"
	"github.com/stretchr/testify/assert"
)

func TestPointDistance(t *testing.T) {
	assert.Equal(t, 5.0, PointDistance(image.Point{0, 3}, image.Point{4, 0}))
}

func TestPointCenter(t *testing.T) {
	all := []image.Point{
		{0, 0},
		{2, 0},
		{0, 2},
		{2, 2},
	}

	assert.Equal(t, image.Point{1, 1}, Center(all, 1000))

	all = append(all, image.Point{100, 100})

	assert.Equal(t, image.Point{50, 50}, Center(all, 1000))
	assert.Equal(t, image.Point{1, 1}, Center(all, 48))

}

func TestPointAngle(t *testing.T) {
	assert.Equal(t, 0.0, PointAngle(image.Point{0, 0}, image.Point{1, 0}))
	assert.Equal(t, math.Pi/4, PointAngle(image.Point{0, 0}, image.Point{1, 1}))
	assert.Equal(t, math.Pi/4-math.Pi, PointAngle(image.Point{0, 0}, image.Point{-1, -1}))
}

func TestPointBoundingBox(t *testing.T) {
	r := BoundingBox([]image.Point{
		{100, 100},
		{200, 200},
		{50, 50},
		{1000, 1000},
		{1, 1},
	})

	assert.Equal(t, image.Point{1, 1}, r.Min)
	assert.Equal(t, image.Point{1000, 1000}, r.Max)
}

func TestR2ToImage(t *testing.T) {
	imagePoint := image.Point{2, 3}

	assert.Equal(t, imagePoint, R2PointToImagePoint(r2.Point{2.36, 3.004}))
	assert.NotEqual(t, imagePoint, R2PointToImagePoint(r2.Point{2.5, 3.5}))

	imageRect := image.Rect(-2, 1, 5, 8)
	r2Rect := r2.RectFromPoints(r2.Point{-2.1, 1.44}, r2.Point{5.33, 8.49})

	assert.Equal(t, imageRect, R2RectToImageRect(r2Rect))

	r2Rect2 := r2.RectFromPoints(r2.Point{-2.5, 1.5}, r2.Point{5.0, 8.49})

	assert.NotEqual(t, imageRect, R2RectToImageRect(r2Rect2))
	assert.Equal(t, image.Rect(-3, 2, 5, 8), R2RectToImageRect(r2Rect2))

	resultImageRect := imageRect.Add(imagePoint)
	resultR2Rect := TranslateR2Rect(r2Rect, r2.Point{2., 3.})

	assert.Equal(t, resultImageRect, R2RectToImageRect(resultR2Rect))

}
