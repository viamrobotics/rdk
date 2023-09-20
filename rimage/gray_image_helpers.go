//go:build !no_cgo

package rimage

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/pkg/errors"
)

// SameImgSize compares image.Grays to see if they're the same size.
func SameImgSize(g1, g2 image.Image) bool {
	if (g1.Bounds().Max.X != g2.Bounds().Max.X) || (g1.Bounds().Max.Y != g2.Bounds().Max.Y) {
		return false
	}
	return true
}

// MakeGray takes a rimage.Image and well... makes it gray (image.Gray).
func MakeGray(pic *Image) *image.Gray {
	// Converting image to grayscale
	result := image.NewGray(pic.Bounds())
	draw.Draw(result, result.Bounds(), pic, pic.Bounds().Min, draw.Src)

	return result
}

// MultiplyGrays takes in two image.Grays and calculates the product. The
// result must go in a image.Gray16 so that the numbers have space to breathe.
func MultiplyGrays(g1, g2 *image.Gray) (*image.Gray16, error) {
	newPic := image.NewGray16(g1.Bounds())
	if !SameImgSize(g1, g2) {
		return nil, errors.Errorf("these images aren't the same size (%d %d) != (%d %d)",
			g1.Bounds().Max.X, g1.Bounds().Max.Y, g2.Bounds().Max.X, g2.Bounds().Max.Y)
	}
	for y := g1.Bounds().Min.Y; y < g1.Bounds().Max.Y; y++ {
		for x := g1.Bounds().Min.X; x < g1.Bounds().Max.X; x++ {
			newval := uint16(g1.At(x, y).(color.Gray).Y) * uint16(g2.At(x, y).(color.Gray).Y)
			newcol := color.Gray16{Y: newval}
			newPic.Set(x, y, newcol)
		}
	}
	return newPic, nil
}

// GetGrayAvg takes in a grayscale image and returns the average value as an int.
func GetGrayAvg(pic *image.Gray16) int {
	var sum int64
	for y := pic.Bounds().Min.Y; y < pic.Bounds().Max.Y; y++ {
		for x := pic.Bounds().Min.X; x < pic.Bounds().Max.X; x++ {
			val := pic.At(x, y).(color.Gray16).Y
			sum += int64(val)
		}
	}
	return int(sum / int64(pic.Bounds().Max.X*pic.Bounds().Max.Y))
}

// GetGraySum takes in a grayscale image and returns the total sum as an int.
func GetGraySum(gray *image.Gray16) int {
	avg := GetGrayAvg(gray)
	return avg * gray.Bounds().Max.X * gray.Bounds().Max.Y
}
