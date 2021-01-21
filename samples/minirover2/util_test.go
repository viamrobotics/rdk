package main

import (
	"strings"
	"testing"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
	"github.com/echolabsinc/robotcore/vision/segmentation"
	
)

type MyDebug struct {
}

func (ddd MyDebug) Process(d *vision.MultipleImageTestDebugger, fn string, img vision.Image) error {
	dm, err := vision.ParseDepthMap(strings.Replace(fn, ".png", ".dat.gz", 1))
	if err != nil {
		return err
	}

	pc := vision.PointCloud{dm, img}

	pc, err = pc.CropToDepthData()
	if err != nil {
		return err
	}
	d.GotDebugImage(pc.Color.MatUnsafe(), "cropped")

	debug2 := gocv.NewMatWithSize(pc.Color.Rows(), pc.Color.Cols(), gocv.MatTypeCV8UC3)
	defer debug2.Close()
	roverWalk(&pc, &debug2)
	d.GotDebugImage(debug2, "depth2")

	return nil
}

func Test1(t *testing.T) {
	d := vision.NewMultipleImageTestDebugger("minirover2/autodrive", "*.png")
	err := d.Process(MyDebug{})
	if err != nil {
		t.Fatal(err)
	}

}

// ----

type ChargeDebug struct {
}

func (cd ChargeDebug) Process(d *vision.MultipleImageTestDebugger, fn string, img vision.Image) error {
	m := img.MatUnsafe()
	gocv.Rotate(m, &m, gocv.Rotate180Clockwise)
	img, err := vision.NewImage(m)
	if err != nil {
		return err
	}

	d.GotDebugImage(m, "rotated")

	m2, err := segmentation.ShapeWalkEntireDebug(img, false)
	if err != nil {
		return err
	}
	d.GotDebugImage(m2, "segments")
	
	
	return nil
}

func TestCharge1(t *testing.T) {
	d := vision.NewMultipleImageTestDebugger("minirover2/charging2", "*.both.gz")
	err := d.Process(ChargeDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
