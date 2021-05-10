package imagesource

import (
	"image"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

type smoothTestHelper struct {
	attrs api.AttributeMap
}

func (h *smoothTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := rimage.ConvertToImageWithDepth(img)

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth")

	dc, err := NewDepthComposed(nil, nil, h.attrs, logger)
	test.That(t, err, test.ShouldBeNil)

	fixed, err := dc.camera.AlignImageWithDepth(ii)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	// use Opening smoothing
	// kernel size 3, 1 iteration
	openedDM, err := rimage.OpeningMorph(fixed.Depth, 3, 1)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(openedDM.ToPrettyPicture(0, rimage.MaxDepth), "depth-opened")

	// use Closing smoothing
	// size 3, 1 iteration
	closedDM1, err := rimage.ClosingMorph(fixed.Depth, 3, 1)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(closedDM1.ToPrettyPicture(0, rimage.MaxDepth), "depth-closed-3-1")
	// size 3, 3 iterations
	closedDM2, err := rimage.ClosingMorph(fixed.Depth, 3, 3)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(closedDM2.ToPrettyPicture(0, rimage.MaxDepth), "depth-closed-3-3")
	// size 5, 1 iteration
	closedDM3, err := rimage.ClosingMorph(fixed.Depth, 5, 1)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(closedDM3.ToPrettyPicture(0, rimage.MaxDepth), "depth-closed-5-1")
	// size 5, 3 iterations
	closedDM4, err := rimage.ClosingMorph(fixed.Depth, 5, 3)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(closedDM4.ToPrettyPicture(0, rimage.MaxDepth), "depth-closed-5-3")

	return nil
}

func TestSmoothGripper(t *testing.T) {
	config, err := api.ReadConfig(utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &smoothTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}
