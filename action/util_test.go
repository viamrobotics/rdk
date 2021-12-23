package action

import (
	"image"
	"strings"
	"testing"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/segmentation"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

type MyDebug struct {
}

func (ddd MyDebug) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img image.Image,
	logger golog.Logger,
) error {
	dm, err := rimage.ParseDepthMap(strings.Replace(fn, ".png", ".dat.gz", 1))
	if err != nil {
		return err
	}

	pc := rimage.MakeImageWithDepth(rimage.ConvertImage(img), dm, false, nil)

	pc, err = pc.CropToDepthData()
	if err != nil {
		return err
	}
	pCtx.GotDebugImage(pc.Color, "cropped")
	pCtx.GotDebugImage(pc.Depth.ToPrettyPicture(0, 0), "cropped-depth")

	walked, _ := roverWalk(pc, true, logger)
	pCtx.GotDebugImage(walked, "depth2")

	return nil
}

func TestAutoDrive1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/autodrive", "*.png", false)
	err := d.Process(t, MyDebug{})
	test.That(t, err, test.ShouldBeNil)

}

type ChargeDebug struct {
}

func (cd ChargeDebug) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img image.Image,
	logger golog.Logger,
) error {
	iwd := rimage.ConvertToImageWithDepth(img).Rotate(180)
	pCtx.GotDebugImage(iwd, "rotated")

	m2, err := segmentation.ShapeWalkEntireDebug(iwd, segmentation.ShapeWalkOptions{}, logger)
	if err != nil {
		return err
	}
	pCtx.GotDebugImage(m2, "segments")

	if iwd.Depth != nil {
		pCtx.GotDebugImage(iwd.Depth.ToPrettyPicture(0, 0), "depth")
	}

	return nil
}

func TestCharge1(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "minirover2/charging2", "*.both.gz", false)
	err := d.Process(t, ChargeDebug{})
	test.That(t, err, test.ShouldBeNil)

}
