package imagesource

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/testutils"
)

type alignTestHelper struct {
	attrs api.AttributeMap
	dc    *DepthComposed
}

func (h *alignTestHelper) Process(d *rimage.MultipleImageTestDebugger, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := rimage.ConvertToImageWithDepth(img)

	d.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth")

	if h.dc == nil {
		h.dc, err = NewDepthComposed(nil, nil, h.attrs, logger)
		if err != nil {
			d.T.Fatal(err)
		}
	}

	fixed, err := h.dc.alignColorAndDepth(context.TODO(), ii, logger)
	if err != nil {
		d.T.Fatal(err)
	}

	d.GotDebugImage(fixed.Color, "color-fixed")
	d.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	d.GotDebugImage(fixed.Overlay(), "overlay")
	return nil
}

func TestAlignIntel(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz")
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

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz")
	err = d.Process(&alignTestHelper{c.Attributes, nil})
	if err != nil {
		t.Fatal(err)
	}
}
