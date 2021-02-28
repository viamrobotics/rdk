package actions

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/vision/segmentation"

	"github.com/disintegration/imaging"
)

type MyDebug struct {
}

func (ddd MyDebug) Process(d *rimage.MultipleImageTestDebugger, fn string, img image.Image) error {
	dm, err := rimage.ParseDepthMap(strings.Replace(fn, ".png", ".dat.gz", 1))
	if err != nil {
		return err
	}

	pc := &rimage.ImageWithDepth{rimage.ConvertImage(img), dm}

	pc, err = pc.CropToDepthData()
	if err != nil {
		return err
	}
	d.GotDebugImage(pc.Color, "cropped")

	walked, _ := roverWalk(pc, true)
	d.GotDebugImage(walked, "depth2")

	return nil
}

func Test1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/autodrive", "*.png")
	err := d.Process(MyDebug{})
	if err != nil {
		t.Fatal(err)
	}

}

// ----

type ChargeDebug struct {
}

func (cd ChargeDebug) Process(d *rimage.MultipleImageTestDebugger, fn string, img image.Image) error {
	img = imaging.Rotate(img, 180, color.Black)
	d.GotDebugImage(img, "rotated")

	m2, err := segmentation.ShapeWalkEntireDebug(rimage.ConvertImage(img), segmentation.ShapeWalkOptions{})
	if err != nil {
		return err
	}
	d.GotDebugImage(m2, "segments")

	return nil
}

func TestCharge1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/charging2", "*.both.gz")
	err := d.Process(ChargeDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
