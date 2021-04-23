package main

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/rimage"
)

type ChargeDebug struct {
}

func (cd ChargeDebug) Process(t *testing.T, d *rimage.MultipleImageTestDebugger, fn string, img image.Image, logger golog.Logger) error {
	i2 := rimage.ConvertImage(img)

	top, x, err := findBlack(context.Background(), i2, logger)
	if err != nil {
		return err
	}
	d.GotDebugImage(x, "foo")

	i2.Circle(top, 5, rimage.Red)
	d.GotDebugImage(i2, "foo2")

	return nil
}

func TestCharge1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/charging3", "*.jpg", false)
	err := d.Process(t, ChargeDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
