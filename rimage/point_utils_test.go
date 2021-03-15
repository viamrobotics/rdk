package rimage

import (
	"image"
	"math"
	"testing"

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
