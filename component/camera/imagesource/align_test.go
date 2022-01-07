package imagesource

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func TestAlignIntrinsics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)
	c := conf.FindComponent("front")
	test.That(t, c, test.ShouldNotBeNil)
	delete(c.Attributes, "warp")
	delete(c.Attributes, "homography")

	ii, err := rimage.ReadBothFromFile(artifact.MustPath("align/intel515/chairs.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	aligned, _ := applyAlignment(t, ii, *c.ConvertedAttributes.(*rimage.AttrConfig), logger)
	test.That(t, aligned, test.ShouldNotBeNil)
}

func TestAlignWarp(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := conf.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	delete(c.Attributes, "intrinsic_extrinsic")
	delete(c.Attributes, "homography")

	ii, err := rimage.ReadBothFromFile(artifact.MustPath("align/gripper1/chess1.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	aligned, _ := applyAlignment(t, ii, *c.ConvertedAttributes.(*rimage.AttrConfig), logger)
	test.That(t, aligned, test.ShouldNotBeNil)
}

func TestAlignHomography(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := conf.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	delete(c.Attributes, "intrinsic_extrinsic")
	delete(c.Attributes, "warp")

	ii, err := rimage.ReadBothFromFile(artifact.MustPath("align/gripper1/chess1.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	aligned, _ := applyAlignment(t, ii, *c.ConvertedAttributes.(*rimage.AttrConfig), logger)
	test.That(t, aligned, test.ShouldNotBeNil)
}

func applyAlignment(
	t *testing.T,
	ii *rimage.ImageWithDepth,
	attrs rimage.AttrConfig,
	logger golog.Logger,
) (*rimage.ImageWithDepth, *depthComposed) {
	t.Helper()
	colorSource := &staticSource{ii.Color}
	depthSource := &staticSource{ii.Depth}
	is, err := NewDepthComposed(colorSource, depthSource, &attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	dc, ok := is.(*depthComposed)
	test.That(t, ok, test.ShouldBeTrue)

	rawAligned, _, err := dc.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	fixed := rimage.ConvertToImageWithDepth(rawAligned)
	return fixed, dc
}

type alignTestHelper struct {
	attrs rimage.AttrConfig
	name  string
}

func (h *alignTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	var err error
	ii := rimage.ConvertToImageWithDepth(img)
	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth_"+h.name)

	fixed, dc := applyAlignment(t, ii, h.attrs, logger)

	pCtx.GotDebugImage(fixed.Color, "color-fixed_"+h.name)
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed_"+h.name)

	pCtx.GotDebugImage(fixed.Overlay(), "overlay_"+h.name)

	// get pointcloud
	fixed.SetProjector(dc.projectionCamera)
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
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front").ConvertedAttributes.(*rimage.AttrConfig)
	test.That(t, c, test.ShouldNotBeNil)

	c.Warp = nil
	c.Homography = nil
	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{*c, "intrinsics"})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignGripperWarp(t *testing.T) {
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined").ConvertedAttributes.(*rimage.AttrConfig)
	test.That(t, c, test.ShouldNotBeNil)

	c.IntrinsicExtrinsic = nil
	c.Homography = nil
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	d.Process(t, &alignTestHelper{*c, "warp"})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignGripperHomography(t *testing.T) {
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined").ConvertedAttributes.(*rimage.AttrConfig)
	test.That(t, c, test.ShouldNotBeNil)

	c.IntrinsicExtrinsic = nil
	c.Warp = nil
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{*c, "homography"})
	test.That(t, err, test.ShouldBeNil)
}
