package chess

import (
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/vision/segmentation"
)

type P func(d *rimage.ImageWithDepth, logger golog.Logger) (image.Image, []image.Point, error)

type ChessImageProcessDebug struct {
	p P
}

func (dd ChessImageProcessDebug) Process(t *testing.T, d *rimage.MultipleImageTestDebugger, fn string, img image.Image, logger golog.Logger) error {
	out, corners, err := dd.p(rimage.ConvertToImageWithDepth(img), logger)
	if err != nil {
		return err
	}

	swOptions := segmentation.ShapeWalkOptions{}
	swOptions.MaxRadius = 50

	d.GotDebugImage(out, "corners")

	if corners != nil {
		warped, err := warpColorAndDepthToChess(rimage.ConvertToImageWithDepth(img), corners)
		if err != nil {
			return err
		}

		d.GotDebugImage(warped.Color, "warped")

		starts := []image.Point{}
		for x := 50; x <= 750; x += 100 {
			for y := 50; y <= 750; y += 100 {
				starts = append(starts, image.Point{x, y})
			}
		}

		res, err := segmentation.ShapeWalkMultiple(warped.Color, starts, swOptions, logger)
		if err != nil {
			return err
		}

		d.GotDebugImage(res, "shapes")

		if true {
			out := rimage.NewImageFromBounds(res.Bounds())
			for idx, p := range starts {
				count := res.PixelsInSegmemnt(idx + 1)
				clr := rimage.Red

				if count > 7000 {
					clr = rimage.Green
				}

				out.Circle(p, 20, clr)

			}

			d.GotDebugImage(out, "marked")
		}

		if warped.Depth != nil {
			d.GotDebugImage(warped.Depth.ToPrettyPicture(0, 10000), "depth1")
			d.GotDebugImage(warped.Overlay(), "depth2")
		}

		if false {
			clusters, err := rimage.ClusterFromImage(warped.Color, 4)
			if err != nil {
				return err
			}

			clustered := rimage.ClusterImage(clusters, warped.Color)

			d.GotDebugImage(clustered, "kmeans")
		}
	}

	return nil
}

func TestChessCheatRed1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "chess/boardseliot2", "*")
	err := d.Process(t, &ChessImageProcessDebug{FindChessCornersPinkCheat})
	if err != nil {
		t.Fatal(err)
	}
}
