package segmentation

import (
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/rimage"
)

type chunkImageDebug struct {
}

func (cid *chunkImageDebug) Process(d *rimage.MultipleImageTestDebugger, fn string, imgraw image.Image, logger golog.Logger) error {

	img := rimage.ConvertImage(imgraw)

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
			out, err := ShapeWalkMultiple(img, starts, ShapeWalkOptions{SkipCleaning: true}, logger)
			if err != nil {
				return err
			}
			d.GotDebugImage(out, "shapes-noclean")
		}

		out, err := ShapeWalkMultiple(img, starts, ShapeWalkOptions{}, logger)
		if err != nil {
			return err
		}

		d.GotDebugImage(out, "shapes")

		for idx, s := range cfg.Shapes {
			numPixels := out.PixelsInSegmemnt(idx + 1)
			if numPixels < s.PixelRange[0] || numPixels > s.PixelRange[1] {
				// run again with debugging on
				_, err := ShapeWalkMultiple(img, []image.Point{s.Start}, ShapeWalkOptions{Debug: true}, logger)
				if err != nil {
					return err
				}

				d.T.Errorf("out of pixel range %s %v %d", fn, s, numPixels)
			}
		}

	}

	if true {
		out, err := ShapeWalkEntireDebug(img, ShapeWalkOptions{}, logger)
		if err != nil {
			return err
		}
		d.GotDebugImage(out, "entire")
	}

	return nil
}

func TestChunk1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/test1", "*")
	err := d.Process(&chunkImageDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
