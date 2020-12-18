package main

import (
	"image"
	"strings"
	"testing"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

type MyDebug struct {
}

func (ddd MyDebug) Process(d *vision.MultipleImageTestDebugger, fn string, mat gocv.Mat) error {
	img, err := vision.NewImage(mat)
	if err != nil {
		return err
	}

	dm, err := vision.ParseDepthMap(strings.Replace(fn, ".png", ".dat.gz", 1))
	if err != nil {
		return err
	}

	pc := vision.PointCloud{dm, img}

	gocv.Rectangle(&mat, image.Rect(
		pc.Width()/2-350, pc.Height()-1,
		pc.Width()/2+350, pc.Height()-450),
		vision.Red.C, 1)
	//d.GotDebugImage(mat, "box")

	pc = pc.CropToDepthData()
	d.GotDebugImage(pc.Color.MatUnsafe(), "cropped")

	debug := gocv.NewMatWithSize(pc.Color.Rows(), pc.Color.Cols(), gocv.MatTypeCV8UC3)
	defer debug.Close()
	roverColorize(&pc, debug)
	d.GotDebugImage(debug, "depth1")

	debug2 := gocv.NewMatWithSize(pc.Color.Rows(), pc.Color.Cols(), gocv.MatTypeCV8UC3)
	defer debug2.Close()
	roverWalk(&pc, debug2)
	d.GotDebugImage(debug2, "depth2")

	return nil
}

func Test1(t *testing.T) {
	d := vision.NewMultipleImageTestDebugger("minirover2", "*.png")
	err := d.Process(MyDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
