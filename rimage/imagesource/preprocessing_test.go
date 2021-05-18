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

	vectorField := rimage.SobelFilter(fixed.Depth)
	pCtx.GotDebugImage(vectorField.MagnitudePicture(), "depth-grad-magnitude")
	pCtx.GotDebugImage(vectorField.DirectionPicture(), "depth-grad-direction")

	vectorField2 := rimage.ForwardGradientDepth(fixed.Depth)
	pCtx.GotDebugImage(vectorField2.MagnitudePicture(), "forward-depth-grad-magnitude")
	pCtx.GotDebugImage(vectorField2.DirectionPicture(), "forward-depth-grad-direction")

	//cannyColor := rimage.NewCannyDericheEdgeDetectorWithParameters(0.7, 0.25, false)
	cannyColor := rimage.NewCannyDericheEdgeDetector()
	cannyDepth := rimage.NewCannyDericheEdgeDetectorWithParameters(0.9, 0.55, false)

	colEdges, err := cannyColor.DetectEdges(fixed.Color, 0.5)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(colEdges, "color-edges-nopreprocess")

	dmEdges, err := cannyDepth.DetectDepthEdges(fixed.Depth)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(dmEdges, "depth-edges-nopreprocess")

	closedDM, err := rimage.ClosingMorph(fixed.Depth, 5, 1)
	test.That(t, err, test.ShouldBeNil)
	morphed := rimage.MakeImageWithDepth(fixed.Color, closedDM, fixed.IsAligned(), fixed.CameraSystem())
	dmClosedEdges, err := cannyDepth.DetectDepthEdges(morphed.Depth)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(morphed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-closed-5-1")
	pCtx.GotDebugImage(dmClosedEdges, "depth-edges-preprocess-1")
	morphedPCD, err := morphed.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(morphedPCD, "morphed-pcd")

	kernelSize := 9
	spatialVar, colorVar, depthVar := 6.0, 15.0, 20.0
	filtered, err := rimage.JointTrilateralFilter(morphed, kernelSize, spatialVar, colorVar, depthVar)
	test.That(t, err, test.ShouldBeNil)
	dmFilteredEdges, err := cannyDepth.DetectDepthEdges(filtered.Depth)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(filtered.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-filtered")
	pCtx.GotDebugImage(dmFilteredEdges, "depth-edges-filtered")
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
