package transform

import (
	"context"
	"image"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
)

type homographyTestHelper struct {
	params *DepthColorHomography
	proj   *PinholeCameraIntrinsics
}

func (h *homographyTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img, img2 image.Image,
	logger logging.Logger,
) error {
	t.Helper()
	var err error
	im := rimage.ConvertImage(img)
	dm, err := rimage.ConvertImageToDepthMap(context.Background(), img2)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "depth_homography")

	imgFixed, dmFixed, err := h.params.AlignColorAndDepthImage(im, dm)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(imgFixed, "color-fixed_homography")
	pCtx.GotDebugImage(dmFixed.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed_homography")

	pCtx.GotDebugImage(rimage.Overlay(imgFixed, dmFixed), "overlay_homography")

	// get pointcloud
	pc, err := h.proj.RGBDToPointCloud(imgFixed, dmFixed)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(pc, "aligned-pointcloud_homography")

	// go back to image and depth map
	roundTripImg, roundTripDm, err := h.proj.PointCloudToRGBD(pc)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(roundTripImg, "from-pointcloud-color")
	pCtx.GotDebugImage(roundTripDm.ToPrettyPicture(0, rimage.MaxDepth), "from-pointcloud-depth")

	return nil
}

func TestNewHomography(t *testing.T) {
	_, err := NewHomography([]float64{})
	test.That(t, err, test.ShouldBeError, errors.New("input to NewHomography must have length of 9. Has length of 0"))

	vals := []float64{
		2.32700501e-01,
		-8.33535395e-03,
		-3.61894025e+01,
		-1.90671303e-03,
		2.35303232e-01,
		8.38582614e+00,
		-6.39101664e-05,
		-4.64582754e-05,
		1.00000000e+00,
	}
	_, err = NewHomography(vals)
	test.That(t, err, test.ShouldBeNil)
}

func TestDepthColorHomography(t *testing.T) {
	intrinsics := &PinholeCameraIntrinsics{ // color camera intrinsic parameters
		Width:  1024,
		Height: 768,
		Fx:     821.32642889,
		Fy:     821.68607359,
		Ppx:    494.95941428,
		Ppy:    370.70529534,
	}
	conf := &RawDepthColorHomography{
		Homography: []float64{
			2.32700501e-01,
			-8.33535395e-03,
			-3.61894025e+01,
			-1.90671303e-03,
			2.35303232e-01,
			8.38582614e+00,
			-6.39101664e-05,
			-4.64582754e-05,
			1.00000000e+00,
		},
		DepthToColor: false,
		RotateDepth:  -90,
	}

	dch, err := NewDepthColorHomography(conf)
	test.That(t, err, test.ShouldBeNil)
	d := rimage.NewMultipleImageTestDebugger(t, "transform/homography/color", "*.png", "transform/homography/depth")
	err = d.Process(t, &homographyTestHelper{dch, intrinsics})
	test.That(t, err, test.ShouldBeNil)
}
