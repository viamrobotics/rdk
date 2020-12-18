package chess

import (
	"fmt"
	"image"
	"testing"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

type P func(gocv.Mat, *gocv.Mat) ([]image.Point, error)

type ChessImageProcessDebug struct {
	p P
}

func (dd ChessImageProcessDebug) Process(d *vision.MultipleImageTestDebugger, fn string, img gocv.Mat) error {
	out := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC3)
	defer out.Close()
	corners, err := dd.p(img, &out)
	if err != nil {
		return err
	}

	fmt.Printf("\t%v %v\n", corners, img.Type())

	d.GotDebugImage(out, "corners")

	if corners != nil {
		warped, _, err := warpColorAndDepthToChess(img, vision.DepthMap{}, corners)
		if err != nil {
			return err
		}

		d.GotDebugImage(warped, "warped")

	}

	return nil
}

func TestChessCheatRed1(t *testing.T) {
	d := vision.NewMultipleImageTestDebugger("chess/boardseliot2", "*.png")
	err := d.Process(&ChessImageProcessDebug{FindChessCornersPinkCheat})
	if err != nil {
		t.Fatal(err)
	}
}
