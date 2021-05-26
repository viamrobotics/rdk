package rimage

import (
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

// Smoothing with Morphological filters
type smoothTestHelper struct{}

func (h *smoothTestHelper) Process(t *testing.T, pCtx *ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := ConvertToImageWithDepth(img)

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, MaxDepth), "depth")

	// use Opening smoothing
	// kernel size 3, 1 iteration
	openedDM, err := OpeningMorph(ii.Depth, 3, 1)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(openedDM.ToPrettyPicture(0, MaxDepth), "depth-opened")

	// use Closing smoothing
	// size 3, 1 iteration
	closedDM1, err := ClosingMorph(ii.Depth, 3, 1)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(closedDM1.ToPrettyPicture(0, MaxDepth), "depth-closed-3-1")
	// size 3, 3 iterations
	closedDM2, err := ClosingMorph(ii.Depth, 3, 3)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(closedDM2.ToPrettyPicture(0, MaxDepth), "depth-closed-3-3")
	// size 5, 1 iteration
	closedDM3, err := ClosingMorph(ii.Depth, 5, 1)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(closedDM3.ToPrettyPicture(0, MaxDepth), "depth-closed-5-1")

	return nil
}

func TestSmoothGripper(t *testing.T) {
	d := NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err := d.Process(t, &smoothTestHelper{})
	test.That(t, err, test.ShouldBeNil)
}

// Canny Edge Detection for depth maps
type cannyTestHelper struct{}

func (h *cannyTestHelper) Process(t *testing.T, pCtx *ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	cannyColor := NewCannyDericheEdgeDetector()
	cannyDepth := NewCannyDericheEdgeDetectorWithParameters(0.85, 0.33, false)

	ii := ConvertToImageWithDepth(img)

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, MaxDepth), "depth-ii")

	// edges no preprocessing
	colEdges, err := cannyColor.DetectEdges(ii.Color, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(colEdges, "color-edges-nopreprocess")

	dmEdges, err := cannyDepth.DetectDepthEdges(ii.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(dmEdges, "depth-edges-nopreprocess")

	// cleaned
	CleanDepthMap(ii.Depth, 500)
	dmCleanedEdges, err := cannyDepth.DetectDepthEdges(ii.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, 500), "depth-cleaned-near")        //near
	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(500, 4000), "depth-cleaned-middle")   // middle
	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(4000, MaxDepth), "depth-cleaned-far") // far
	pCtx.GotDebugImage(dmCleanedEdges, "depth-edges-cleaned")

	// morphological
	closedDM, err := ClosingMorph(ii.Depth, 5, 1)
	test.That(t, err, test.ShouldBeNil)
	morphed := MakeImageWithDepth(ii.Color, closedDM, ii.IsAligned(), ii.CameraSystem())
	dmClosedEdges, err := cannyDepth.DetectDepthEdges(morphed.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(morphed.Depth.ToPrettyPicture(0, MaxDepth), "depth-closed-5-1")
	pCtx.GotDebugImage(dmClosedEdges, "depth-edges-preprocess-1")

	// color code the distances of the missing data
	pCtx.GotDebugImage(DrawAverageHoleDepth(morphed.Depth), "hole-depths")

	// filled
	FillDepthMap(morphed.Depth)
	filledEdges, err := cannyDepth.DetectDepthEdges(morphed.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(morphed.Depth.ToPrettyPicture(0, MaxDepth), "depth-holes-filled")
	pCtx.GotDebugImage(filledEdges, "depth-edges-filled")

	//smoothed
	smoothDM := GaussianSmoothing(morphed.Depth, 1)
	smoothed := MakeImageWithDepth(morphed.Color, smoothDM, ii.IsAligned(), ii.CameraSystem())
	dmSmoothedEdges, err := cannyDepth.DetectDepthEdges(smoothed.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(smoothed.Depth.ToPrettyPicture(0, MaxDepth), "depth-smoothed")
	pCtx.GotDebugImage(dmSmoothedEdges, "depth-edges-smoothed")

	//depth smoothed
	bilateralDM := JointBilateralSmoothing(morphed.Depth, 1, 500)
	bilateral := MakeImageWithDepth(morphed.Color, bilateralDM, ii.IsAligned(), ii.CameraSystem())
	dmBilateralEdges, err := cannyDepth.DetectDepthEdges(bilateral.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(bilateral.Depth.ToPrettyPicture(0, MaxDepth), "depth-bilateral")
	pCtx.GotDebugImage(dmBilateralEdges, "depth-edges-bilateral")

	//savitsky-golay smoothed
	sgDM := NewEmptyDepthMap(morphed.Depth.Width(), morphed.Depth.Height())
	validPoints := MissingDepthData(morphed.Depth)
	err = SavitskyGolaySmoothing(morphed.Depth, sgDM, validPoints, 3, 3)
	test.That(t, err, test.ShouldBeNil)
	sg := MakeImageWithDepth(morphed.Color, sgDM, ii.IsAligned(), ii.CameraSystem())
	sgEdges, err := cannyDepth.DetectDepthEdges(sg.Depth, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(sg.Depth.ToPrettyPicture(0, MaxDepth), "depth-savitskygolay")
	pCtx.GotDebugImage(sgEdges, "depth-edges-savitskygolay")

	vectorField := ForwardDepthGradient(smoothed.Depth)
	pCtx.GotDebugImage(vectorField.MagnitudePicture(), "depth-grad-magnitude")
	pCtx.GotDebugImage(vectorField.DirectionPicture(), "depth-grad-direction")
	return nil
}

func TestCannyGripper(t *testing.T) {
	d := NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err := d.Process(t, &cannyTestHelper{})
	test.That(t, err, test.ShouldBeNil)
}

func TestCannyIntel(t *testing.T) {
	d := NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err := d.Process(t, &cannyTestHelper{})
	test.That(t, err, test.ShouldBeNil)
}
