package imagesource

import (
	"context"
	"image"
	"testing"

	"go.viam.com/core/config"
	"go.viam.com/core/rimage"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

type alignTestHelper struct {
	attrs config.AttributeMap
	name  string
}

func (h *alignTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := rimage.ConvertToImageWithDepth(img)
	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth_"+h.name)

	colorSource := &staticSource{ii.Color}
	depthSource := &staticSource{ii.Depth}
	is, err := NewDepthComposed(colorSource, depthSource, h.attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	dc, ok := is.(*depthComposed)
	test.That(t, ok, test.ShouldBeTrue)

	rawAligned, _, err := dc.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	fixed := rimage.ConvertToImageWithDepth(rawAligned)
	pCtx.GotDebugImage(fixed.Color, "color-fixed_"+h.name)
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed_"+h.name)

	pCtx.GotDebugImage(fixed.Overlay(), "overlay_"+h.name)

	// get pointcloud
	fixed.SetCameraSystem(dc.projectionCamera)
	pc, err := fixed.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(pc, "aligned-pointcloud_"+h.name)

	// go back to image with depth
	roundTrip, err := dc.projectionCamera.PointCloudToImageWithDepth(pc)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(roundTrip.Overlay(), "from-pointcloud_"+h.name)

	return nil
}

func TestAlignIntelIntrinsics(t *testing.T) {
	config, err := config.Read(utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front")
	test.That(t, c, test.ShouldNotBeNil)

	delete(c.Attributes, "warp")
	delete(c.Attributes, "homography")
	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c.Attributes, "intrinsics"})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignGripperWarp(t *testing.T) {
	config, err := config.Read(utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	delete(c.Attributes, "intrinsic_extrinsic")
	delete(c.Attributes, "homography")
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c.Attributes, "warp"})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignGripperHomography(t *testing.T) {
	config, err := config.Read(utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	delete(c.Attributes, "intrinsic_extrinsic")
	delete(c.Attributes, "warp")
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c.Attributes, "homography"})
	test.That(t, err, test.ShouldBeNil)
}
