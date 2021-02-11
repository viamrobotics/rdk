package segmentation

import (
	"image"
	"testing"

	"github.com/viamrobotics/robotcore/vision"
)

type chunkImageDebug struct {
}

func (cid *chunkImageDebug) Process(d *vision.MultipleImageTestDebugger, fn string, img vision.Image) error {

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
		out, err := ShapeWalkEntireDebug(img, false)
		if err != nil {
			return err
		}
		d.GotDebugImage(out, "entire")
	}

	if true {
		starts := []image.Point{}

		for _, s := range cfg.Shapes {
			starts = append(starts, s.Start)
		}

		out, err := ShapeWalkMultiple(img, starts, false)
		if err != nil {
			return err
		}

		d.GotDebugImage(out, "shapes")

		for idx, s := range cfg.Shapes {
			numPixels := out.PixelsInSegmemnt(idx + 1)
			if numPixels < s.PixelRange[0] || numPixels > s.PixelRange[1] {
				// run again with debugging on
				_, err := ShapeWalkMultiple(img, starts, true)
				if err != nil {
					return err
				}

				d.T.Errorf("out of pixel range %v %d", s, numPixels)
			}
		}

	}

	return nil
}

func TestChunk1(t *testing.T) {
	d := vision.NewMultipleImageTestDebugger(t, "segmentation/test1", "*")
	err := d.Process(&chunkImageDebug{})
	if err != nil {
		t.Fatal(err)
	}

}
