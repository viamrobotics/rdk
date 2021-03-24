package main

import (
	"image"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/rimage"
)

type ChargeDebug struct {
}

func (cd ChargeDebug) Process(d *rimage.MultipleImageTestDebugger, fn string, img image.Image, logger golog.Logger) error {
	return nil
}

func TestCharge1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/charging3", "*.jpg")
	err := d.Process(ChargeDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
