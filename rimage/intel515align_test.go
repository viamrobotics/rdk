package rimage

import (
	"image"
	"testing"
)

type intelTestHelper struct {
}

func (h *intelTestHelper) Process(d *MultipleImageTestDebugger, fn string, img image.Image) error {
	ii := ConvertToImageWithDepth(img)

	d.GotDebugImage(ii.Depth.ToPrettyPicture(0, 1000000), "depth")

	a := Intel515Align{}
	fixed, err := a.Align(ii)
	if err != nil {
		d.T.Fatal(err)
	}

	d.GotDebugImage(fixed.Color, "color-fixed")
	d.GotDebugImage(fixed.Depth.ToPrettyPicture(0, 1000000), "depth-fixed")

	d.GotDebugImage(fixed.Overlay(), "overlay")
	return nil
}

func TestAlignMultiple(t *testing.T) {
	d := NewMultipleImageTestDebugger(t, "intel515alginment", "*.both.gz")
	err := d.Process(&intelTestHelper{})
	if err != nil {
		t.Fatal(err)
	}

}
