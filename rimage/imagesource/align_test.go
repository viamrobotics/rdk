package imagesource

import (
	"image"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/rimage/transform"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

type alignTestHelper struct {
	attrs api.AttributeMap
}

func (h *alignTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := rimage.ConvertToImageWithDepth(img)

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth")

	dc, err := NewDepthComposed(nil, nil, h.attrs, logger)
	test.That(t, err, test.ShouldBeNil)

	fixed, err := dc.camera.AlignImageWithDepth(ii)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(fixed.Color, "color-fixed")
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	pCtx.GotDebugImage(fixed.Overlay(), "overlay")

	pc, err := fixed.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	roundTrip, err := dc.camera.PointCloudToImageWithDepth(pc)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(roundTrip.Overlay(), "from-pointcloud")

	return nil
}

func TestAlignIntelWarp(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515_warp", "*.both.gz", false)
	err := d.Process(t, &alignTestHelper{api.AttributeMap{"config": &transform.IntelConfig}})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignIntelMatrices(t *testing.T) {
	config, err := api.ReadConfig(utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignGripper(t *testing.T) {
	config, err := api.ReadConfig(utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}
