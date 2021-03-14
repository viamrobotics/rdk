package rimage

import (
	"context"
	"image"
	"testing"

	"go.viam.com/robotcore/api"
)

type alignTestHelper struct {
}

func (h *alignTestHelper) Process(d *MultipleImageTestDebugger, fn string, img image.Image) error {
	ii := ConvertToImageWithDepth(img)

	d.GotDebugImage(ii.Depth.ToPrettyPicture(0, MaxDepth), "depth")

	dc, err := NewDepthComposed(nil, nil, api.AttributeMap{"config": &intelConfig})
	if err != nil {
		d.T.Fatal(err)
	}

	fixed, err := dc.alignColorAndDepth(context.TODO(), ii)
	if err != nil {
		d.T.Fatal(err)
	}

	d.GotDebugImage(fixed.Color, "color-fixed")
	d.GotDebugImage(fixed.Depth.ToPrettyPicture(0, MaxDepth), "depth-fixed")

	d.GotDebugImage(fixed.Overlay(), "overlay")
	return nil
}

func TestAlignMultiple(t *testing.T) {
	d := NewMultipleImageTestDebugger(t, "intel515alginment", "*.both.gz")
	err := d.Process(&alignTestHelper{})
	if err != nil {
		t.Fatal(err)
	}

}
