package chess

import (
	"image"
	"testing"

	"github.com/echolabsinc/robotcore/vision"
)

type P func(vision.Image) (image.Image, []image.Point, error)

type ChessImageProcessDebug struct {
	p P
}

func (dd ChessImageProcessDebug) Process(d *vision.MultipleImageTestDebugger, fn string, img vision.Image) error {
	out, corners, err := dd.p(img)
	if err != nil {
		return err
	}

	d.GotDebugImage(out, "corners")

	if corners != nil {
		warped, _, err := warpColorAndDepthToChess(img, &vision.DepthMap{}, corners)
		if err != nil {
			return err
		}

		warpedImg, err := warped.ToImage()
		if err != nil {
			return err
		}
		d.GotDebugImage(warpedImg, "warped")

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
