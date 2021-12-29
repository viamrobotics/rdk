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
	source := &staticSource{img}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(0.85, 0.40, true)
	blur := 3.0
	ds := &depthEdgesSource{source, canny, blur}
	_, _, err = ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

type depthSourceTestHelper struct {
	attrs config.AttributeMap
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
	is, err := NewDepthComposed(nil, nil, h.attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	dc, ok := is.(*depthComposed)
	test.That(t, ok, test.ShouldBeTrue)
	fixed, err := dc.alignmentCamera.AlignImageWithDepth(ii)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "aligned-depth")

	// change to use projection camera
	fixed.SetProjector(dc.projectionCamera)
	// create edge map
	source := &staticSource{fixed}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(0.85, 0.40, true)
	blur := 3.0
	ds := &depthEdgesSource{source, canny, blur}
	edges, _, err := ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(edges, "edges-aligned-depth")

	// make point cloud
	fixedPointCloud, err := fixed.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(fixedPointCloud, "aligned-pointcloud")

	// preprocess depth map
	source = &staticSource{fixed}
	rs := &preprocessDepthSource{source}

	output, _, err := rs.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	preprocessed := rimage.ConvertToImageWithDepth(output)

	pCtx.GotDebugImage(preprocessed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "preprocessed-aligned-depth")
	preprocessedPointCloud, err := preprocessed.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(preprocessedPointCloud, "preprocessed-aligned-pointcloud")

	source = &staticSource{preprocessed}
	ds = &depthEdgesSource{source, canny, blur}
	processedEdges, _, err := ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(processedEdges, "edges-preprocessed-aligned-depth")

	return nil
}

func TestDepthSourceGripper(t *testing.T) {
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &depthSourceTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}

func TestDepthSourceIntel(t *testing.T) {
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &depthSourceTestHelper{c.Attributes})
	test.That(t, err, test.ShouldBeNil)
}
