package rimage

import (
	"context"
	"image"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/testutils"
)

type alignTestHelper struct {
	attrs api.AttributeMap
	dc    *DepthComposed
}

func (h *alignTestHelper) Process(d *MultipleImageTestDebugger, fn string, img image.Image) error {
	var err error
	ii := ConvertToImageWithDepth(img)

	d.GotDebugImage(ii.Depth.ToPrettyPicture(0, MaxDepth), "depth")

	if h.dc == nil {
		h.dc, err = NewDepthComposed(nil, nil, h.attrs)
		if err != nil {
			d.T.Fatal(err)
		}
	}

	fixed, err := h.dc.alignColorAndDepth(context.TODO(), ii)
	if err != nil {
		d.T.Fatal(err)
	}

	d.GotDebugImage(fixed.Color, "color-fixed")
	d.GotDebugImage(fixed.Depth.ToPrettyPicture(0, MaxDepth), "depth-fixed")

	d.GotDebugImage(fixed.Overlay(), "overlay")
	return nil
}

func TestAlignIntel(t *testing.T) {
	d := NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz")
	err := d.Process(&alignTestHelper{api.AttributeMap{"config": &intelConfig}, nil})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAlignGripper(t *testing.T) {
	config, err := api.ReadConfig(testutils.ResolveFile("robots/configs/gripper-cam.json"))
	if err != nil {
		t.Fatal(err)
	}

	c := config.FindComponent("combined")
	if c == nil {
		t.Fatal("no combined")
	}

	d := NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz")
	err = d.Process(&alignTestHelper{c.Attributes, nil})
	if err != nil {
		t.Fatal(err)
	}
}
