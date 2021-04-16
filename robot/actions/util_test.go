package actions

import (
	"image"
	"strings"
	"testing"

	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/vision/segmentation"

	"github.com/edaniels/golog"
)

type MyDebug struct {
}

func (ddd MyDebug) Process(t *testing.T, d *rimage.MultipleImageTestDebugger, fn string, img image.Image, logger golog.Logger) error {
	dm, err := rimage.ParseDepthMap(strings.Replace(fn, ".png", ".dat.gz", 1))
	if err != nil {
		return err
	}

	pc := rimage.MakeImageWithDepth(rimage.ConvertImage(img), dm, false)

	pc, err = pc.CropToDepthData()
	if err != nil {
		return err
	}
	d.GotDebugImage(pc.Color, "cropped")

	walked, _ := roverWalk(pc, true, logger)
	d.GotDebugImage(walked, "depth2")

	return nil
}

func Test1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/autodrive", "*.png")
	err := d.Process(t, MyDebug{})
	if err != nil {
		t.Fatal(err)
	}

}

// ----

type ChargeDebug struct {
}

func (cd ChargeDebug) Process(t *testing.T, d *rimage.MultipleImageTestDebugger, fn string, img image.Image, logger golog.Logger) error {
	iwd := rimage.ConvertToImageWithDepth(img).Rotate(180)
	d.GotDebugImage(iwd, "rotated")

	m2, err := segmentation.ShapeWalkEntireDebug(iwd.Color, segmentation.ShapeWalkOptions{}, logger)
	if err != nil {
		return err
	}
	d.GotDebugImage(m2, "segments")

	if iwd.Depth != nil {
		d.GotDebugImage(iwd.Depth.ToPrettyPicture(0, 10000), "depth")
	}

	return nil
}

func TestCharge1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/charging2", "*.both.gz")
	err := d.Process(t, ChargeDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
