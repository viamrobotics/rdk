package utils

import (
	"image"
	"image/color"
	"testing"

	"go.viam.com/test"
)

func TestParallelForEachPixel(t *testing.T) {
	imGray := image.NewGray(image.Rect(0, 0, 100, 200))
	originalSize := imGray.Bounds().Size()
	ParallelForEachPixel(originalSize, func(x int, y int) {
		val, _, _, _ := imGray.At(x, y).RGBA()
		imGray.Set(x, y, color.Gray{uint8(val + 1)})
	})

	test.That(t, imGray.At(0, 0), test.ShouldResemble, color.Gray{1})
}
