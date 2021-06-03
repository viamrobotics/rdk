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
	depthImg := ii.Depth

	pCtx.GotDebugImage(depthImg.ToPrettyPicture(0, MaxDepth), "depth-ii")

	// edges no preprocessing
	colEdges, err := cannyColor.DetectEdges(ii.Color, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(colEdges, "color-edges-nopreprocess")

	dmEdges, err := cannyDepth.DetectDepthEdges(depthImg, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(dmEdges, "depth-edges-nopreprocess")

	// cleaned
	CleanDepthMap(depthImg)
	dmCleanedEdges, err := cannyDepth.DetectDepthEdges(depthImg, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(depthImg.ToPrettyPicture(0, 500), "depth-cleaned-near")        //near
	pCtx.GotDebugImage(depthImg.ToPrettyPicture(500, 4000), "depth-cleaned-middle")   // middle
	pCtx.GotDebugImage(depthImg.ToPrettyPicture(4000, MaxDepth), "depth-cleaned-far") // far
	pCtx.GotDebugImage(dmCleanedEdges, "depth-edges-cleaned")

	// morphological
	closedDM, err := ClosingMorph(depthImg, 5, 1)
	test.That(t, err, test.ShouldBeNil)
	dmClosedEdges, err := cannyDepth.DetectDepthEdges(closedDM, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(closedDM.ToPrettyPicture(0, MaxDepth), "depth-closed-5-1")
	pCtx.GotDebugImage(dmClosedEdges, "depth-edges-preprocess-1")

	// color code the distances of the missing data
	pCtx.GotDebugImage(drawAverageHoleDepth(closedDM), "hole-depths")

	// filled
	morphed := MakeImageWithDepth(ii.Color, closedDM, ii.IsAligned(), ii.CameraSystem())
	FillDepthMap(morphed)
	closedDM = morphed.Depth
	filledEdges, err := cannyDepth.DetectDepthEdges(closedDM, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(closedDM.ToPrettyPicture(0, MaxDepth), "depth-holes-filled")
	pCtx.GotDebugImage(filledEdges, "depth-edges-filled")

	//smoothed
	smoothDM, err := GaussianSmoothing(closedDM, 1)
	test.That(t, err, test.ShouldBeNil)
	dmSmoothedEdges, err := cannyDepth.DetectDepthEdges(smoothDM, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(smoothDM.ToPrettyPicture(0, MaxDepth), "depth-smoothed")
	pCtx.GotDebugImage(dmSmoothedEdges, "depth-edges-smoothed")

	//bilateral smoothed
	bilateralDM, err := JointBilateralSmoothing(closedDM, 1, 500)
	test.That(t, err, test.ShouldBeNil)
	dmBilateralEdges, err := cannyDepth.DetectDepthEdges(bilateralDM, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(bilateralDM.ToPrettyPicture(0, MaxDepth), "depth-bilateral")
	pCtx.GotDebugImage(dmBilateralEdges, "depth-edges-bilateral")

	//savitsky-golay smoothed
	validPoints := MissingDepthData(closedDM)
	sgDM, err := SavitskyGolaySmoothing(closedDM, validPoints, 3, 3)
	test.That(t, err, test.ShouldBeNil)
	sgEdges, err := cannyDepth.DetectDepthEdges(sgDM, 0.0)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(sgDM.ToPrettyPicture(0, MaxDepth), "depth-savitskygolay")
	pCtx.GotDebugImage(sgEdges, "depth-edges-savitskygolay")

	return nil
}

func TestDepthPreprocessCanny(t *testing.T) {
	d := NewMultipleImageTestDebugger(t, "depthpreprocess", "*both.gz", true)
	err := d.Process(t, &cannyTestHelper{})
	test.That(t, err, test.ShouldBeNil)
}

// Depth pre-processing pipeline
type preprocessTestHelper struct{}

func (h *preprocessTestHelper) Process(t *testing.T, pCtx *ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	var err error
	ii := ConvertToImageWithDepth(img)
	depthImg := ii.Depth

	pCtx.GotDebugImage(depthImg.ToPrettyPicture(0, MaxDepth), "depth-raw")
	pCtx.GotDebugImage(ii.Overlay(), "raw-overlay")

	missingDepth := MissingDepthData(depthImg)
	pCtx.GotDebugImage(missingDepth, "depth-raw-missing-data")

	pc, err := ii.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(pc, "raw-depth-pointcloud")

	preprocessedIwd, err := PreprocessDepthMap(ii)
	preprocessedImg := preprocessedIwd.Depth
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(preprocessedImg.ToPrettyPicture(0, MaxDepth), "depth-preprocessed")
	pCtx.GotDebugImage(preprocessedIwd.Overlay(), "preprocessed-overlay")

	missingPreprocessDepth := MissingDepthData(preprocessedImg)
	pCtx.GotDebugImage(missingPreprocessDepth, "depth-preprocessed-missing-data")

	pc2, err := preprocessedIwd.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(pc2, "preprocess-depth-pointcloud")

	return nil
}

func TestDepthPreprocess(t *testing.T) {
	d := NewMultipleImageTestDebugger(t, "depthpreprocess", "*.both.gz", true)
	err := d.Process(t, &preprocessTestHelper{})
	test.That(t, err, test.ShouldBeNil)
}

// drawAverageHoleDepth is a debugging function to see the depth calculated by averageDepthAroundHole
func drawAverageHoleDepth(dm *DepthMap) *Image {
	red, green, blue := NewColor(255, 0, 0), NewColor(0, 255, 0), NewColor(0, 0, 255)
	img := NewImage(dm.Width(), dm.Height())
	validData := MissingDepthData(dm)
	missingData := invertGrayImage(validData)
	holeMap := segmentBinaryImage(missingData)
	for _, seg := range holeMap {
		borderPoints := getPointsOnHoleBorder(seg, dm)
		avgDepth := averageDepthInSegment(borderPoints, dm)
		var c Color
		switch {
		case avgDepth < 500.0:
			c = red
		case avgDepth >= 500.0 && avgDepth < 4000.0:
			c = green
		default:
			c = blue
		}
		for pt := range seg {
			img.Set(pt, c)
		}
	}
	return img
}
