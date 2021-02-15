package chess

import (
	"image"
	"testing"

	"github.com/viamrobotics/robotcore/vision"
	"github.com/viamrobotics/robotcore/vision/segmentation"
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

	swOptions := segmentation.ShapeWalkOptions{}
	swOptions.MaxRadius = 50

	d.GotDebugImage(out, "corners")

	if corners != nil {
		warped, _, err := warpColorAndDepthToChess(img, &vision.DepthMap{}, corners)
		if err != nil {
			return err
		}

		d.GotDebugImage(warped.Image(), "warped")

		starts := []image.Point{}
		for x := 50; x <= 750; x += 100 {
			for y := 50; y <= 750; y += 100 {
				starts = append(starts, image.Point{x, y})
			}
		}

		res, err := segmentation.ShapeWalkMultiple(warped, starts, swOptions)
		if err != nil {
			return err
		}

		d.GotDebugImage(res, "shapes")

		if false {
			clusters, err := warped.ClusterHSV(4)
			if err != nil {
				return err
			}

			clustered := vision.ClusterImage(clusters, warped)

			d.GotDebugImage(clustered, "kmeans")
		}
	}

	return nil
}

func TestChessCheatRed1(t *testing.T) {
	d := vision.NewMultipleImageTestDebugger(t, "chess/boardseliot2", "*.png")
	err := d.Process(&ChessImageProcessDebug{FindChessCornersPinkCheat})
	if err != nil {
		t.Fatal(err)
	}
}
