package imagesource

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func TestDepthSource(t *testing.T) {
	img, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)
	source := &StaticSource{img}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(0.85, 0.40, true)
	blur := 3.0
	ds := &depthEdgesSource{source, canny, blur}
	_, _, err = ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

type depthSourceTestHelper struct {
	attrs *alignAttrs
}

func (h *depthSourceTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	ii := rimage.ConvertToImageWithDepth(img)
	// align the images
	aligner, err := getAligner(h.attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	fixedColor, fixedDepth, err := aligner.AlignColorAndDepthImage(ii.Color, ii.Depth)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(fixedDepth.ToPrettyPicture(0, rimage.MaxDepth), "aligned-depth")

	// change to use projection camera
	// create edge map
	source := &StaticSource{fixedDepth}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(0.85, 0.40, true)
	blur := 3.0
	ds := &depthEdgesSource{source, canny, blur}
	edges, _, err := ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(edges, "edges-aligned-depth")

	// make point cloud
	fixedPointCloud, err := h.attrs.CameraParameters.RGBDToPointCloud(fixedColor, fixedDepth)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(fixedPointCloud, "aligned-pointcloud")

	// preprocess depth map
	source = &StaticSource{fixedDepth}
	rs := &preprocessDepthSource{source}

	output, _, err := rs.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	preprocessed := rimage.ConvertToImageWithDepth(output)

	pCtx.GotDebugImage(preprocessed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "preprocessed-aligned-depth")
	preprocessedPointCloud, err := h.attrs.CameraParameters.RGBDToPointCloud(preprocessed.Color, preprocessed.Depth)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(preprocessedPointCloud, "preprocessed-aligned-pointcloud")

	source = &StaticSource{preprocessed}
	ds = &depthEdgesSource{source, canny, blur}
	processedEdges, _, err := ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(processedEdges, "edges-preprocessed-aligned-depth")

	return nil
}

func TestDepthSourceGripper(t *testing.T) {
	logger := golog.NewTestLogger(t)
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined").ConvertedAttributes.(*alignAttrs)
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &depthSourceTestHelper{c})
	test.That(t, err, test.ShouldBeNil)
}

func TestDepthSourceIntel(t *testing.T) {
	logger := golog.NewTestLogger(t)
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/intel.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front").ConvertedAttributes.(*alignAttrs)
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &depthSourceTestHelper{c})
	test.That(t, err, test.ShouldBeNil)
}
