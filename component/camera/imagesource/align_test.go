package imagesource

import (
	"context"
	"image"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

const debugImageSource = "VIAM_DEBUG"

func debugImageSourceOrSkip(t *testing.T) {
	t.Helper()
	imageSourceTest := os.Getenv(debugImageSource)
	if imageSourceTest == "" {
		t.Skipf("set environmental variable %q to run this test", debugImageSource)
	}
}

func TestAlignTypeError(t *testing.T) {
	logger := golog.NewTestLogger(t)
	im, err := rimage.NewImageFromFile(artifact.MustPath("align/intel515/chairs_color.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(artifact.MustPath("align/intel515/chairs.png"))
	test.That(t, err, test.ShouldBeNil)
	colorSrc := &StaticSource{ColorImg: im}
	colorCam, err := camera.New(colorSrc, nil)
	test.That(t, err, test.ShouldBeNil)
	depthSrc := &StaticSource{DepthImg: dm}
	depthCam, err := camera.New(depthSrc, nil)
	test.That(t, err, test.ShouldBeNil)
	attrs := &alignAttrs{
		AttrConfig: &camera.AttrConfig{
			Width:  100,
			Height: 200,
		},
	}
	// test Warp error
	attrs.Warp = []float64{4.5, 6.}
	_, err = newAlignColorDepth(context.Background(), colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeError, utils.NewUnexpectedTypeError(&transform.AlignConfig{}, attrs.Warp))
	// test Homography error
	attrs.Warp = nil
	attrs.Homography = 4
	_, err = newAlignColorDepth(context.Background(), colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeError, utils.NewUnexpectedTypeError(&transform.RawDepthColorHomography{}, attrs.Homography))
	// test Extrinsics errors
	attrs.Homography = nil
	attrs.IntrinsicExtrinsic = "a"
	_, err = newAlignColorDepth(context.Background(), colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeError, utils.NewUnexpectedTypeError(&transform.DepthColorIntrinsicsExtrinsics{}, attrs.IntrinsicExtrinsic))
	// test no types
	attrs.IntrinsicExtrinsic = nil
	_, err = newAlignColorDepth(context.Background(), colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeNil)
}

func applyAlignment(
	t *testing.T,
	img *rimage.Image,
	dm *rimage.DepthMap,
	attrs *alignAttrs,
	logger golog.Logger,
) (pointcloud.PointCloud, rimage.Projector) {
	t.Helper()
	colorSrc := &StaticSource{ColorImg: img}
	colorCam, err := camera.New(colorSrc, nil)
	test.That(t, err, test.ShouldBeNil)
	depthSrc := &StaticSource{DepthImg: dm}
	depthCam, err := camera.New(depthSrc, nil)
	test.That(t, err, test.ShouldBeNil)
	is, err := newAlignColorDepth(context.Background(), colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeNil)

	alignedPointCloud, err := is.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	proj, err := is.GetProperties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	return alignedPointCloud, proj
}

// nolint:dupl
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
	attrs.Height = 720
	attrs.Width = 1280

	im, err := rimage.NewImageFromFile(artifact.MustPath("align/intel515/chairs_color.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(artifact.MustPath("align/intel515/chairs.png"))
	test.That(t, err, test.ShouldBeNil)
	aligned, _ := applyAlignment(t, im, dm, attrs, logger)
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
	attrs.Height = 342
	attrs.Width = 448

	warpParams, err := transform.NewPinholeCameraIntrinsicsFromJSONFile(
		utils.ResolveFile("robots/configs/gripper_combo_parameters.json"), "color",
	)
	test.That(t, err, test.ShouldBeNil)
	attrs.CameraParameters = warpParams

	im, err := rimage.NewImageFromFile(artifact.MustPath("align/gripper1/chess1_color.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(artifact.MustPath("align/gripper1/chess1.png"))
	test.That(t, err, test.ShouldBeNil)
	aligned, _ := applyAlignment(t, im, dm, attrs, logger)
	test.That(t, aligned, test.ShouldNotBeNil)
}

// nolint:dupl
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
	attrs.Height = 768
	attrs.Width = 1024
	im, err := rimage.NewImageFromFile(artifact.MustPath("align/intel515/chairs_color.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(artifact.MustPath("align/intel515/chairs.png"))
	test.That(t, err, test.ShouldBeNil)
	aligned, _ := applyAlignment(t, im, dm, attrs, logger)
	test.That(t, aligned, test.ShouldNotBeNil)
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
	img2 image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	var err error
	im := rimage.ConvertImage(img)
	dm, err := rimage.ConvertImageToDepthMap(img2)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "depth_"+h.name)

	aligned, intrinsics := applyAlignment(t, im, dm, h.attrs, logger)
	fixedColor, fixedDepth, err := intrinsics.PointCloudToRGBD(aligned)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(fixedColor, "color-fixed_"+h.name)
	pCtx.GotDebugImage(fixedDepth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed_"+h.name)

	pCtx.GotDebugImage(rimage.Overlay(fixedColor, fixedDepth), "overlay_"+h.name)

	// convert back to pointcloud again and compare
	pc, err := h.attrs.CameraParameters.RGBDToPointCloud(fixedColor, fixedDepth)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(pc, "aligned-pointcloud_"+h.name)
	test.That(t, pc.Size(), test.ShouldEqual, aligned.Size())

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
	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515/color", "*.png", "align/intel515/depth")
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
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1/color", "*.png", "align/gripper1/depth")
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
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1/color", "*.png", "align/gripper1/depth")
	err = d.Process(t, &alignTestHelper{c, "homography"})
	test.That(t, err, test.ShouldBeNil)
}
