package imagetransform

import (
	"context"
	"image"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/camera/imagesource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

const debugImageTransform = "VIAM_DEBUG"

func debugImageTransformOrSkip(t *testing.T) {
	t.Helper()
	imageTransformTest := os.Getenv(debugImageTransform)
	if imageTransformTest == "" {
		t.Skipf("set environmental variable %q to run this test", debugImageTransform)
	}
}

func TestDepthSource(t *testing.T) {
	img, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1.dat.gz"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{DepthImg: img}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(0.85, 0.40, true)
	blur := 3.0
	ds := &depthEdgesSource{source, canny, blur}
	_, _, err = ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

type depthSourceTestHelper struct {
	proj rimage.Projector
}

func (h *depthSourceTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	dm, err := rimage.ConvertImageToDepthMap(img)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "aligned-depth")

	// create edge map
	source := &imagesource.StaticSource{DepthImg: dm}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(0.85, 0.40, true)
	blur := 3.0
	ds := &depthEdgesSource{source, canny, blur}
	edges, _, err := ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(edges, "edges-aligned-depth")

	// make point cloud
	fixedPointCloud := dm.ToPointCloud(h.proj)
	pCtx.GotDebugPointCloud(fixedPointCloud, "aligned-pointcloud")

	// preprocess depth map
	source = &imagesource.StaticSource{DepthImg: dm}
	rs := &preprocessDepthSource{source}

	output, _, err := rs.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	preprocessed, err := rimage.ConvertImageToDepthMap(output)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(preprocessed.ToPrettyPicture(0, rimage.MaxDepth), "preprocessed-aligned-depth")
	preprocessedPointCloud := preprocessed.ToPointCloud(h.proj)
	pCtx.GotDebugPointCloud(preprocessedPointCloud, "preprocessed-aligned-pointcloud")

	source = &imagesource.StaticSource{DepthImg: preprocessed}
	ds = &depthEdgesSource{source, canny, blur}
	processedEdges, _, err := ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(processedEdges, "edges-preprocessed-aligned-depth")

	return nil
}

func TestDepthSourceGripper(t *testing.T) {
	debugImageTransformOrSkip(t)
	logger := golog.NewTestLogger(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/gripper-cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("combined").ConvertedAttributes.(*camera.AttrConfig)
	test.That(t, c, test.ShouldNotBeNil)
	proj := c.CameraParameters

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz", false)
	err = d.Process(t, &depthSourceTestHelper{proj})
	test.That(t, err, test.ShouldBeNil)
}

func TestDepthSourceIntel(t *testing.T) {
	debugImageTransformOrSkip(t)
	logger := golog.NewTestLogger(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/intel.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front").ConvertedAttributes.(*camera.AttrConfig)
	test.That(t, c, test.ShouldNotBeNil)
	proj := c.CameraParameters

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz", false)
	err = d.Process(t, &depthSourceTestHelper{proj})
	test.That(t, err, test.ShouldBeNil)
}
