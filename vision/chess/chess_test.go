package chess

import (
	"image"
	"testing"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

type P func(vision.Image, *gocv.Mat) ([]image.Point, error)

type ChessImageProcessDebug struct {
	p P
}

func (dd ChessImageProcessDebug) Process(d *vision.MultipleImageTestDebugger, fn string, img vision.Image) error {
	out := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC3)
	defer out.Close()
	corners, err := dd.p(img, &out)
	if err != nil {
		return err
	}

	d.GotDebugImage(out, "corners")

	if corners != nil {
		warped, _, err := warpColorAndDepthToChess(img, vision.DepthMap{}, corners)
		if err != nil {
			return err
		}

		d.GotDebugImage(warped.MatUnsafe(), "warped")

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
