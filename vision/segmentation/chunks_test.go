package segmentation

import (
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/rimage"
)

type chunkImageDebug struct {
}

func (cid *chunkImageDebug) Process(
	t *testing.T,
	d *rimage.MultipleImageTestDebugger,
	fn string,
	imgraw image.Image,
	logger golog.Logger) error {

	iwd := rimage.ConvertToImageWithDepth(imgraw)
	img := iwd.Color

	type AShape struct {
		Start      image.Point
		PixelRange []int
	}

	type imgConfig struct {
		Shapes []AShape
	}

	cfg := imgConfig{}
	err := d.CurrentImgConfig(&cfg)
	if err != nil {
		return err
	}

	if true {
		out := img.InterestingPixels(.2)
		d.GotDebugImage(out, "t")
	}

	if true {
		starts := []image.Point{}

		for _, s := range cfg.Shapes {
			starts = append(starts, s.Start)
		}

		if true {
			// this shows things with the cleaning, is it useful, not sure
			out, err := ShapeWalkMultiple(iwd, starts, ShapeWalkOptions{SkipCleaning: true}, logger)
			if err != nil {
				return err
			}
			d.GotDebugImage(out, "shapes-noclean")
		}

		out, err := ShapeWalkMultiple(iwd, starts, ShapeWalkOptions{}, logger)
		if err != nil {
			return err
		}

		d.GotDebugImage(out, "shapes")

		for idx, s := range cfg.Shapes {
			numPixels := out.PixelsInSegmemnt(idx + 1)
			if numPixels < s.PixelRange[0] || numPixels > s.PixelRange[1] {
				// run again with debugging on
				_, err := ShapeWalkMultiple(iwd, []image.Point{s.Start}, ShapeWalkOptions{Debug: true}, logger)
				if err != nil {
					return err
				}

				t.Errorf("out of pixel range %s %v %d", fn, s, numPixels)
			}
		}

	}

	if true {
		out, err := ShapeWalkEntireDebug(iwd, ShapeWalkOptions{}, logger)
		if err != nil {
			return err
		}
		d.GotDebugImage(out, "entire")
	}

	if iwd.Depth != nil {
		x := iwd.Depth.ToPrettyPicture(0, 0)
		d.GotDebugImage(x, "depth")
	}

	return nil
}

func TestChunk1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/test1", "*", true)
	err := d.Process(t, &chunkImageDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
