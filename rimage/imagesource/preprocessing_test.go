package imagesource

import (
	"image"
	"testing"

	"go.viam.com/core/config"
	"go.viam.com/core/rimage"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

// Smoothing with Morphological filters
type smoothTestHelper struct {
	attrs config.AttributeMap
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

	return nil
}

func TestSmoothGripper(t *testing.T) {
	config, err := config.Read(utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &smoothTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}

// Canny Edge Detection for depth maps
type cannyTestHelper struct {
	attrs config.AttributeMap
}

func (h *cannyTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := rimage.ConvertToImageWithDepth(img)

	dc, err := NewDepthComposed(nil, nil, h.attrs, logger)
	test.That(t, err, test.ShouldBeNil)

	fixed, err := dc.camera.AlignImageWithDepth(ii)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(fixed.Color, "color-fixed")
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	holes := rimage.MissingDepthData(fixed.Depth)
	pCtx.GotDebugImage(holes, "depth-holes")

	rimage.CleanDepthMap(fixed.Depth, 500)
	cleanedHoles := rimage.MissingDepthData(fixed.Depth)
	pCtx.GotDebugImage(cleanedHoles, "depth-holes-cleaned")

	//blurredVectorField := vectorField.Blur(2)
	//pCtx.GotDebugImage(blurredVectorField.MagnitudePicture(), "depth-grad-magnitude-blur")
	//pCtx.GotDebugImage(blurredVectorField.DirectionPicture(), "depth-grad-direction-blur")

	cannyColor := rimage.NewCannyDericheEdgeDetector()
	cannyDepth := rimage.NewCannyDericheEdgeDetectorWithParameters(0.85, 0.33, true)

	colEdges, err := cannyColor.DetectEdges(fixed.Color, 0.5)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(colEdges, "color-edges-nopreprocess")

	/*
		dirPicture := rimage.ConvertImage(vectorField2.DirectionPicture())
		dirPicture = rimage.ConvertImage(imaging.Blur(dirPicture, 3.0))
		pCtx.GotDebugImage(dirPicture, "color-blur")
		colEdges2, err := cannyColor.DetectEdges(dirPicture, 1.5)
		test.That(t, err, test.ShouldBeNil)
		pCtx.GotDebugImage(colEdges2, "color-edges-preprocess")
	*/

	dmEdges, err := cannyDepth.DetectDepthEdges(fixed.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(dmEdges, "depth-edges-nopreprocess")

	// morphological
	closedDM, err := rimage.ClosingMorph(fixed.Depth, 5, 1)
	test.That(t, err, test.ShouldBeNil)
	morphed := rimage.MakeImageWithDepth(fixed.Color, closedDM, fixed.IsAligned(), fixed.CameraSystem())
	dmClosedEdges, err := cannyDepth.DetectDepthEdges(morphed.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(morphed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-closed-5-1")
	pCtx.GotDebugImage(dmClosedEdges, "depth-edges-preprocess-1")

	// filling
	rimage.FillDepthMap(morphed.Depth, 4500)
	filledEdges, err := cannyDepth.DetectDepthEdges(morphed.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(morphed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-holes-filled")
	pCtx.GotDebugImage(filledEdges, "depth-edges-filled")

	//smoothed
	smoothDM := rimage.GaussianBlur(morphed.Depth, 1)
	smoothed := rimage.MakeImageWithDepth(morphed.Color, smoothDM, fixed.IsAligned(), fixed.CameraSystem())
	dmSmoothedEdges, err := cannyDepth.DetectDepthEdges(smoothed.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(smoothed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-smoothed")
	pCtx.GotDebugImage(dmSmoothedEdges, "depth-edges-smoothed")

	//savitsky-golay
	sgDM := rimage.NewEmptyDepthMap(morphed.Depth.Width(), morphed.Depth.Height())
	validPoints := rimage.MissingDepthData(morphed.Depth)
	err = rimage.SavitskyGolaySmoothing(morphed.Depth, sgDM, validPoints, 3, 3)
	test.That(t, err, test.ShouldBeNil)
	sg := rimage.MakeImageWithDepth(morphed.Color, sgDM, fixed.IsAligned(), fixed.CameraSystem())
	sgEdges, err := cannyDepth.DetectDepthEdges(sg.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(sg.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-savitskygolay")
	pCtx.GotDebugImage(sgEdges, "depth-edges-savitskygolay")

	//vectorField := rimage.ForwardDepthGradient(sg.Depth)
	//pCtx.GotDebugImage(vectorField.MagnitudePicture(), "depth-grad-magnitude")
	//pCtx.GotDebugImage(vectorField.DirectionPicture(), "depth-grad-direction")

	// trilateral filter
	/*
		spatialVar, colorVar, depthVar := 1.0, 1.0, 20.0
		filtered := rimage.JointTrilateralSmoothing(morphed, spatialVar, colorVar, depthVar)
		test.That(t, err, test.ShouldBeNil)
		dmFilteredEdges, err := cannyDepth.DetectDepthEdges(filtered.Depth, 0.0)
		test.That(t, err, test.ShouldBeNil)
		pCtx.GotDebugImage(filtered.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-filtered")
		pCtx.GotDebugImage(dmFilteredEdges, "depth-edges-filtered")
	*/
	return nil
}

func TestCannyEdgeGripper(t *testing.T) {
	config, err := config.Read(utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &cannyTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}

func TestCannyEdgeIntel(t *testing.T) {
	config, err := config.Read(utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &cannyTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}
