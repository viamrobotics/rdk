package utils

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"

	"github.com/fogleman/gg"

	"github.com/Ernyoke/Imger/edgedetection"
)

func WriteImageToFile(path string, img image.Image) error {
	if !strings.HasSuffix(path, ".png") {
		return fmt.Errorf("utils.WriteImageToFile can only write to .png for now")
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func ReadImageFromFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func DrawRectangleEmpty(dc *gg.Context, r image.Rectangle, c color.Color, width float64) {
	dc.SetColor(c)

	dc.DrawLine(float64(r.Min.X), float64(r.Min.Y), float64(r.Max.X), float64(r.Min.Y))
	dc.SetLineWidth(width)
	dc.Stroke()

	dc.DrawLine(float64(r.Min.X), float64(r.Min.Y), float64(r.Min.X), float64(r.Max.Y))
	dc.SetLineWidth(width)
	dc.Stroke()

	dc.DrawLine(float64(r.Max.X), float64(r.Min.Y), float64(r.Max.X), float64(r.Max.Y))
	dc.SetLineWidth(width)
	dc.Stroke()

	dc.DrawLine(float64(r.Min.X), float64(r.Max.Y), float64(r.Max.X), float64(r.Max.Y))
	dc.SetLineWidth(width)
	dc.Stroke()
}

func Canny(img image.Image, t1, t2 float64, blurSize uint) (*image.Gray, error) {
	switch i := img.(type) {
	case *image.RGBA:
		return edgedetection.CannyRGBA(i, t1, t2, blurSize)
	case *image.Gray:
		return edgedetection.CannyGray(i, t1, t2, blurSize)
	default:
		return nil, fmt.Errorf("utils.Canny can't handle image type: %t", img)
	}
}

func CountBrightSpots(img *image.Gray, center image.Point, radius int, threshold uint8) int {
	num := 0

	for x := center.X - radius; x < center.X+radius; x++ {
		for y := center.Y - radius; y < center.Y+radius; y++ {
			d := img.GrayAt(x, y)
			if d.Y >= threshold {
				num++
			}
		}
	}

	return num
}
