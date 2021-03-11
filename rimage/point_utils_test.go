package rimage

import (
	"image"
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
