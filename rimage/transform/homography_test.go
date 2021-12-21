package transform

import (
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/rimage"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

type homographyTestHelper struct {
	params *PinholeCameraHomography
}

func (h *homographyTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := rimage.ConvertToImageWithDepth(img)
	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth_homography")

	fixed, err := h.params.AlignImageWithDepth(ii)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(fixed.Color, "color-fixed_homography")
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed_homography")

	pCtx.GotDebugImage(fixed.Overlay(), "overlay_homography")

	// get pointcloud
	pc, err := fixed.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(pc, "aligned-pointcloud_homography")

	// go back to image with depth
	roundTrip, err := h.params.PointCloudToImageWithDepth(pc)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(roundTrip.Overlay(), "from-pointcloud_homography")

	return nil
}

func TestNewHomography(t *testing.T) {
	_, err := NewHomography([]float64{})
	test.That(t, err, test.ShouldBeError, errors.New("input to NewHomography must have length of 9. Has length of 0"))

	vals := []float64{2.32700501e-01, -8.33535395e-03, -3.61894025e+01, -1.90671303e-03, 2.35303232e-01, 8.38582614e+00, -6.39101664e-05, -4.64582754e-05, 1.00000000e+00}
	_, err = NewHomography(vals)
	test.That(t, err, test.ShouldBeNil)
}

func TestPinholeCameraHomography(t *testing.T) {
	intrinsics := PinholeCameraIntrinsics{ // color camera intrinsic parameters
		Width:      1024,
		Height:     768,
		Fx:         821.32642889,
		Fy:         821.68607359,
		Ppx:        494.95941428,
		Ppy:        370.70529534,
		Distortion: DistortionModel{0.11297234, -0.21375332, -0.01584774, -0.00302002, 0.19969297},
	}

	conf := &RawPinholeCameraHomography{
		ColorCamera:  intrinsics,
		Homography:   []float64{2.32700501e-01, -8.33535395e-03, -3.61894025e+01, -1.90671303e-03, 2.35303232e-01, 8.38582614e+00, -6.39101664e-05, -4.64582754e-05, 1.00000000e+00},
		DepthToColor: false,
		RotateDepth:  -90,
	}

	dch, err := NewPinholeCameraHomography(conf)
	test.That(t, err, test.ShouldBeNil)
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &homographyTestHelper{dch})
	test.That(t, err, test.ShouldBeNil)
}
