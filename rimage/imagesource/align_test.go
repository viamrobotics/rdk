// +build !race

package imagesource

import (
	"image"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/rimage/calib"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

type alignTestHelper struct {
	attrs api.AttributeMap
}

func (h *alignTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := rimage.ConvertToImageWithDepth(img)

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth")

	dc, err := NewDepthComposed(nil, nil, h.attrs, logger)
	if err != nil {
		t.Fatal(err)
	}

	fixed, err := dc.aligner.AlignImageWithDepth(ii)
	if err != nil {
		t.Fatal(err)
	}

	pCtx.GotDebugImage(fixed.Color, "color-fixed")
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	pCtx.GotDebugImage(fixed.Overlay(), "overlay")

	pc, err := fixed.ToPointCloud()
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := dc.aligner.PointCloudToImageWithDepth(pc)
	if err != nil {
		t.Fatal(err)
	}
	pCtx.GotDebugImage(roundTrip.Overlay(), "from-pointcloud")

	return nil
}

func TestAlignIntelWarp(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515_warp", "*.both.gz", false)
	err := d.Process(t, &alignTestHelper{api.AttributeMap{"config": &calib.IntelConfig}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAlignIntelMatrices(t *testing.T) {
	config, err := api.ReadConfig(utils.ResolveFile("robots/configs/intel.json"))
	if err != nil {
		t.Fatal(err)
	}

	c := config.FindComponent("front")
	if c == nil {
		t.Fatal("no front")
	}

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c.Attributes})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAlignGripper(t *testing.T) {
	config, err := api.ReadConfig(utils.ResolveFile("robots/configs/gripper-cam.json"))
	if err != nil {
		t.Fatal(err)
	}

	c := config.FindComponent("combined")
	if c == nil {
		t.Fatal("no combined")
	}

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c.Attributes})
	if err != nil {
		t.Fatal(err)
	}
}
