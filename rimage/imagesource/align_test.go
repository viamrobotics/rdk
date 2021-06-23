package imagesource

import (
	"context"
	"image"
	"testing"

	"go.viam.com/utils"

	"go.viam.com/core/config"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

type alignTestHelper struct {
	attrs config.AttributeMap
}

func (h *alignTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := rimage.ConvertToImageWithDepth(img)
	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth")

	colorSource := &StaticSource{ii.Color}
	depthSource := &StaticSource{ii.Depth}
	dc, err := NewDepthComposed(colorSource, depthSource, h.attrs, logger)
	test.That(t, err, test.ShouldBeNil)

	rawAligned, _, err := dc.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	fixed := rimage.ConvertToImageWithDepth(rawAligned)
	pCtx.GotDebugImage(fixed.Color, "color-fixed")
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	pCtx.GotDebugImage(fixed.Overlay(), "overlay")

	// get pointcloud
	fixed.SetCameraSystem(dc.projectionCamera)
	pc, err := fixed.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(pc, "aligned-pointcloud")

	// go back to image with depth
	roundTrip, err := dc.projectionCamera.PointCloudToImageWithDepth(pc)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(roundTrip.Overlay(), "from-pointcloud")

	return nil
}

func TestAlignIntelWarp(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515_warp", "*.both.gz", false)
	err := d.Process(t, &alignTestHelper{config.AttributeMap{"config": &transform.IntelConfig}})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignIntelMatrices(t *testing.T) {
	config, err := config.Read(utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignGripper(t *testing.T) {
	config, err := config.Read(utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}
