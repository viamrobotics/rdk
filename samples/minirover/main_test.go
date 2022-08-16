package main

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/rimage"
)

type ChargeDebug struct{}

func (cd ChargeDebug) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img, img2 image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	i2 := rimage.ConvertImage(img)

	top, x, err := findBlack(context.Background(), i2, logger)
	if err != nil {
		return err
	}
	pCtx.GotDebugImage(x, "foo")

	i2.Circle(top, 5, rimage.Red)
	pCtx.GotDebugImage(i2, "foo2")

	return nil
}

func TestCharge1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/charging3", "*.jpg", "")
	err := d.Process(t, ChargeDebug{})
	test.That(t, err, test.ShouldBeNil)
}
