package main

import (
	"fmt"
	"image"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/vision/segmentation"
)

func findBlack(img *rimage.Image) (image.Image, error) {
	for x := 1; x < img.Width(); x += 3 {
		for y := 1; y < img.Height(); y += 3 {
			c := img.GetXY(x, y)
			if c.Distance(rimage.Black) > 1 {
				continue
			}

			x, err := segmentation.ShapeWalk(img,
				image.Point{x, y},
				segmentation.ShapeWalkOptions{
					//Debug: true,
					ThresholdMod: 1.5,
				},
				golog.Global,
			)
			if err != nil {
				return nil, err
			}

			if x.PixelsInSegmemnt(1) > 5000 {
				return x, nil
			}
		}
	}

	return nil, fmt.Errorf("no black found")
}

type ChargeDebug struct {
}

func (cd ChargeDebug) Process(d *rimage.MultipleImageTestDebugger, fn string, img image.Image, logger golog.Logger) error {

	x, err := findBlack(rimage.ConvertImage(img))
	if err != nil {
		return err
	}
	d.GotDebugImage(x, "foo")

	return nil
}

func TestCharge1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/charging3", "*.jpg")
	err := d.Process(ChargeDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
