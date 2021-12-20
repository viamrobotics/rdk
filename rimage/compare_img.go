package rimage

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/pkg/errors"
)

// IterateImage visits each point in the image and calls the given visitor function
// that can control whether or not to continue iteration.
func IterateImage(img image.Image, f func(x, y int, c color.Color) bool) {
	rect := img.Bounds()
	for x := 0; x < rect.Dx(); x++ {
		for y := 0; y < rect.Dy(); y++ {
			if cont := f(x, y, img.At(x, y)); !cont {
				return
			}
		}
	}
}

// CompareImages compares two images and returns a value of how close they are to being equal
// where zero is equal and the higher it gets, the less like each other they are.
// https://stackoverflow.com/a/60631079/830628
func CompareImages(img1, img2 image.Image) (int, image.Image, error) {
	bounds1 := img1.Bounds()
	bounds2 := img2.Bounds()
	if bounds1 != bounds2 {
		return int(math.MaxInt32), nil, errors.Errorf("image bounds not equal: %+v, %+v", img1.Bounds(), img2.Bounds())
	}

	accumError := int(0)
	resultImg := image.NewRGBA(image.Rect(
		bounds1.Min.X,
		bounds1.Min.Y,
		bounds1.Max.X,
		bounds1.Max.Y,
	))
	draw.Draw(resultImg, resultImg.Bounds(), img1, image.Point{0, 0}, draw.Src)

	for x := bounds1.Min.X; x < bounds1.Max.X; x++ {
		for y := bounds1.Min.Y; y < bounds1.Max.Y; y++ {
			r1, g1, b1, a1 := img1.At(x, y).RGBA()
			r2, g2, b2, a2 := img2.At(x, y).RGBA()

			diff := int(sqDiffUInt32(r1, r2))
			diff += int(sqDiffUInt32(g1, g2))
			diff += int(sqDiffUInt32(b1, b2))
			diff += int(sqDiffUInt32(a1, a2))

			if diff > 0 {
				accumError += diff
				resultImg.Set(
					bounds1.Min.X+x,
					bounds1.Min.Y+y,
					color.RGBA{R: 255, A: 255})
			}
		}
	}

	return int(math.Sqrt(float64(accumError))), resultImg, nil
}

func sqDiffUInt32(x, y uint32) uint64 {
	d := uint64(x) - uint64(y)
	return d * d
}
