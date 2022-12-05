package videosource

import (
	"context"
	"image"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

const debugVideoSource = "VIAM_DEBUG"

func debugVideoSourceOrSkip(t *testing.T) {
	t.Helper()
	videoSourceTest := os.Getenv(debugVideoSource)
	if videoSourceTest == "" {
		t.Skipf("set environmental variable %q to run this test", debugVideoSource)
	}
}

func applyAlignment(
	t *testing.T,
	img *rimage.Image,
	dm *rimage.DepthMap,
	attrs *alignAttrs,
	logger golog.Logger,
) (pointcloud.PointCloud, transform.Projector) {
	t.Helper()
	colorSrc := &StaticSource{ColorImg: img}
	colorCam, err := camera.NewFromReader(context.Background(), colorSrc, nil, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	depthSrc := &StaticSource{DepthImg: dm}
	depthCam, err := camera.NewFromReader(context.Background(), depthSrc, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	is, err := newAlignColorDepth(context.Background(), colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeNil)

	alignedPointCloud, err := is.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	proj, err := is.Projector(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, colorCam.Close(context.Background()), test.ShouldBeNil)
	test.That(t, depthCam.Close(context.Background()), test.ShouldBeNil)
	return alignedPointCloud, proj
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
	dm, err := rimage.ConvertImageToDepthMap(context.Background(), img2)
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
	debugVideoSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("components/camera/videosource/data/intel.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front").ConvertedAttributes.(*alignAttrs)
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515/color", "*.png", "align/intel515/depth")
	err = d.Process(t, &alignTestHelper{c, "intrinsic_parameters"})
	test.That(t, err, test.ShouldBeNil)
}

func TestAlignGripperHomography(t *testing.T) {
	logger := golog.NewTestLogger(t)
	debugVideoSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("components/camera/videosource/data/gripper_cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined").ConvertedAttributes.(*alignAttrs)
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1/color", "*.png", "align/gripper1/depth")
	err = d.Process(t, &alignTestHelper{c, "homography"})
	test.That(t, err, test.ShouldBeNil)
}
