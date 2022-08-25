package transformpipeline

import (
	"context"
	"image"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera/imagesource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
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
	img, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1_gray.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{DepthImg: img}
	am := config.AttributeMap{
		"high_threshold": 0.85,
		"low_threshold":  0.40,
		"blur_radius":    3.0,
	}
	ds, err := newDepthEdgesTransform(source, am)
	test.That(t, err, test.ShouldBeNil)
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
	img2 image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	dm, err := rimage.ConvertImageToDepthMap(img)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "aligned-depth")

	// create edge map
	source := &imagesource.StaticSource{DepthImg: dm}
	am := config.AttributeMap{
		"high_threshold": 0.85,
		"low_threshold":  0.40,
		"blur_radius":    3.0,
	}
	ds, err := newDepthEdgesTransform(source, am)
	test.That(t, err, test.ShouldBeNil)
	edges, _, err := ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(edges, "edges-aligned-depth")

	// make point cloud
	fixedPointCloud := dm.ToPointCloud(h.proj)
	test.That(t, fixedPointCloud.MetaData().HasColor, test.ShouldBeFalse)
	pCtx.GotDebugPointCloud(fixedPointCloud, "aligned-pointcloud")

	// preprocess depth map
	source = &imagesource.StaticSource{DepthImg: dm}
	rs, err := newDepthPreprocessTransform(source)
	test.That(t, err, test.ShouldBeNil)

	output, _, err := rs.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	preprocessed, err := rimage.ConvertImageToDepthMap(output)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(preprocessed.ToPrettyPicture(0, rimage.MaxDepth), "preprocessed-aligned-depth")
	preprocessedPointCloud := preprocessed.ToPointCloud(h.proj)
	test.That(t, preprocessedPointCloud.MetaData().HasColor, test.ShouldBeFalse)
	pCtx.GotDebugPointCloud(preprocessedPointCloud, "preprocessed-aligned-pointcloud")

	source = &imagesource.StaticSource{DepthImg: preprocessed}
	ds, err = newDepthEdgesTransform(source, am)
	test.That(t, err, test.ShouldBeNil)
	processedEdges, _, err := ds.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(processedEdges, "edges-preprocessed-aligned-depth")

	return nil
}

func TestDepthSourceGripper(t *testing.T) {
	debugImageTransformOrSkip(t)
	proj, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/gripper_parameters.json"))
	test.That(t, err, test.ShouldBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1/depth", "*.png", "")
	err = d.Process(t, &depthSourceTestHelper{proj})
	test.That(t, err, test.ShouldBeNil)
}

func TestDepthSourceIntel(t *testing.T) {
	debugImageTransformOrSkip(t)
	proj, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
	test.That(t, err, test.ShouldBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515/depth", "*.png", "")
	err = d.Process(t, &depthSourceTestHelper{proj})
	test.That(t, err, test.ShouldBeNil)
}
