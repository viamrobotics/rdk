package main

import (
	"image"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/vision/segmentation"
)

type ChargeDebug struct {
}

func (cd ChargeDebug) Process(d *rimage.MultipleImageTestDebugger, fn string, img image.Image, logger golog.Logger) error {
	m2, err := segmentation.ShapeWalkEntireDebug(rimage.ConvertImage(img), segmentation.ShapeWalkOptions{}, logger)
	if err != nil {
		return err
	}
	d.GotDebugImage(m2, "segments")

	return nil
}

func TestCharge1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/charging3", "*.jpg")
	err := d.Process(ChargeDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
