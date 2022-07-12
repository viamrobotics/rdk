package imagesource

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func TestAlignTypeError(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ii, err := rimage.ReadBothFromFile(artifact.MustPath("align/intel515/chairs.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	colorSrc := &StaticSource{ii.Color}
	colorCam, err := camera.New(colorSrc, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	depthSrc := &StaticSource{ii.Depth}
	depthCam, err := camera.New(depthSrc, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	attrs := &alignAttrs{}
	// test Warp error
	attrs.Warp = []float64{4.5, 6.}
	_, err = newAlignColorDepth(colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeError, utils.NewUnexpectedTypeError(&transform.AlignConfig{}, attrs.Warp))
	// test Homography error
	attrs.Warp = nil
	attrs.Homography = 4
	_, err = newAlignColorDepth(colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeError, utils.NewUnexpectedTypeError(&transform.RawDepthColorHomography{}, attrs.Homography))
	// test Extrinsics errors
	attrs.Homography = nil
	attrs.IntrinsicExtrinsic = "a"
	_, err = newAlignColorDepth(colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeError, utils.NewUnexpectedTypeError(&transform.DepthColorIntrinsicsExtrinsics{}, attrs.IntrinsicExtrinsic))
	// test no types error
	attrs.IntrinsicExtrinsic = nil
	_, err = newAlignColorDepth(colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeError, errors.New("no valid alignment attribute field provided"))
}

func TestAlignIntrinsics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/intel.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	c := conf.FindComponent("front")
	test.That(t, c, test.ShouldNotBeNil)

	attrs := c.ConvertedAttributes.(*alignAttrs)
	test.That(t, attrs, test.ShouldNotBeNil)
	attrs.Warp = nil
	attrs.Homography = nil

	ii, err := rimage.ReadBothFromFile(artifact.MustPath("align/intel515/chairs.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	aligned := applyAlignment(t, ii, attrs, logger)
	test.That(t, aligned, test.ShouldNotBeNil)
}

func TestAlignWarp(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := conf.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	attrs := c.ConvertedAttributes.(*alignAttrs)
	test.That(t, attrs, test.ShouldNotBeNil)
	attrs.IntrinsicExtrinsic = nil
	attrs.Homography = nil

	warpParams, err := transform.NewPinholeCameraIntrinsicsFromJSONFile(
		utils.ResolveFile("robots/configs/gripper_combo_parameters.json"), "color",
	)
	test.That(t, err, test.ShouldBeNil)
	attrs.CameraParameters = warpParams

	ii, err := rimage.ReadBothFromFile(artifact.MustPath("align/gripper1/chess1.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	aligned := applyAlignment(t, ii, attrs, logger)
	test.That(t, aligned, test.ShouldNotBeNil)
}

func TestAlignHomography(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := conf.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	attrs := c.ConvertedAttributes.(*alignAttrs)
	test.That(t, attrs, test.ShouldNotBeNil)
	attrs.IntrinsicExtrinsic = nil
	attrs.Warp = nil

	ii, err := rimage.ReadBothFromFile(artifact.MustPath("align/gripper1/chess1.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	aligned := applyAlignment(t, ii, attrs, logger)
	test.That(t, aligned, test.ShouldNotBeNil)
}

func applyAlignment(
	t *testing.T,
	ii *rimage.ImageWithDepth,
	attrs *alignAttrs,
	logger golog.Logger,
) *rimage.ImageWithDepth {
	t.Helper()
	colorSrc := &StaticSource{ii.Color}
	colorCam, err := camera.New(colorSrc, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	depthSrc := &StaticSource{ii.Depth}
	depthCam, err := camera.New(depthSrc, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	is, err := newAlignColorDepth(colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeNil)

	rawAligned, _, err := is.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	fixed := rimage.ConvertToImageWithDepth(rawAligned)
	return fixed
}

type alignTestHelper struct {
	attrs *alignAttrs
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

	fixed := applyAlignment(t, ii, h.attrs, logger)

	pCtx.GotDebugImage(fixed.Color, "color-fixed_"+h.name)
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed_"+h.name)

	pCtx.GotDebugImage(fixed.Overlay(), "overlay_"+h.name)

	// get pointcloud
	pc, err := h.attrs.CameraParameters.RGBDToPointCloud(fixed.Color, fixed.Depth)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(pc, "aligned-pointcloud_"+h.name)

	// go back to image with depth
	roundTripColor, roundTripDepth, err := h.attrs.CameraParameters.PointCloudToRGBD(pc)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(rimage.Overlay(roundTripColor, roundTripDepth), "from-pointcloud_"+h.name)

	return nil
}

func TestAlignIntelIntrinsics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/intel.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front").ConvertedAttributes.(*alignAttrs)
	test.That(t, c, test.ShouldNotBeNil)

	c.Warp = nil
	c.Homography = nil
	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c, "intrinsics"})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignGripperWarp(t *testing.T) {
	logger := golog.NewTestLogger(t)
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined").ConvertedAttributes.(*alignAttrs)
	test.That(t, c, test.ShouldNotBeNil)

	c.IntrinsicExtrinsic = nil
	c.Homography = nil
	warpParams, err := transform.NewPinholeCameraIntrinsicsFromJSONFile(
		utils.ResolveFile("robots/configs/gripper_combo_parameters.json"), "color",
	)
	test.That(t, err, test.ShouldBeNil)
	c.CameraParameters = warpParams
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	d.Process(t, &alignTestHelper{c, "warp"})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignGripperHomography(t *testing.T) {
	logger := golog.NewTestLogger(t)
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined").ConvertedAttributes.(*alignAttrs)
	test.That(t, c, test.ShouldNotBeNil)

	c.IntrinsicExtrinsic = nil
	c.Warp = nil
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &alignTestHelper{c, "homography"})
	test.That(t, err, test.ShouldBeNil)
}
