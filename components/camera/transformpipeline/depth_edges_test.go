package transformpipeline

import (
	"context"
	"image"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

const debugVideoTransform = "VIAM_DEBUG"

func debugVideoTransformOrSkip(t *testing.T) {
	t.Helper()
	VideoTransformTest := os.Getenv(debugVideoTransform)
	if VideoTransformTest == "" {
		t.Skipf("set environmental variable %q to run this test", debugVideoTransform)
	}
}

func TestDepthSource(t *testing.T) {
	img, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1_gray_small.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &videosource.StaticSource{DepthImg: img}
	am := config.AttributeMap{
		"high_threshold": 0.85,
		"low_threshold":  0.40,
		"blur_radius":    3.0,
	}
	ds, err := newDepthEdgesTransform(context.Background(), gostream.NewVideoSource(source, prop.Video{}), am)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = camera.ReadImage(context.Background(), ds)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.Close(context.Background()), test.ShouldBeNil)
}

type depthSourceTestHelper struct {
	proj transform.Projector
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
	source := &videosource.StaticSource{DepthImg: dm}
	am := config.AttributeMap{
		"high_threshold": 0.85,
		"low_threshold":  0.40,
		"blur_radius":    3.0,
	}
	ds, err := newDepthEdgesTransform(context.Background(), gostream.NewVideoSource(source, prop.Video{}), am)
	test.That(t, err, test.ShouldBeNil)
	edges, _, err := camera.ReadImage(context.Background(), ds)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.Close(context.Background()), test.ShouldBeNil)

	pCtx.GotDebugImage(edges, "edges-aligned-depth")

	// make point cloud
	fixedPointCloud := depthadapter.ToPointCloud(dm, h.proj)
	test.That(t, fixedPointCloud.MetaData().HasColor, test.ShouldBeFalse)
	pCtx.GotDebugPointCloud(fixedPointCloud, "aligned-pointcloud")

	// preprocess depth map
	source = &videosource.StaticSource{DepthImg: dm}
	rs, err := newDepthPreprocessTransform(context.Background(), gostream.NewVideoSource(source, prop.Video{}))
	test.That(t, err, test.ShouldBeNil)

	output, _, err := camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	preprocessed, err := rimage.ConvertImageToDepthMap(output)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	pCtx.GotDebugImage(preprocessed.ToPrettyPicture(0, rimage.MaxDepth), "preprocessed-aligned-depth")
	preprocessedPointCloud := depthadapter.ToPointCloud(preprocessed, h.proj)
	test.That(t, preprocessedPointCloud.MetaData().HasColor, test.ShouldBeFalse)
	pCtx.GotDebugPointCloud(preprocessedPointCloud, "preprocessed-aligned-pointcloud")

	source = &videosource.StaticSource{DepthImg: preprocessed}
	ds, err = newDepthEdgesTransform(context.Background(), gostream.NewVideoSource(source, prop.Video{}), am)
	test.That(t, err, test.ShouldBeNil)
	processedEdges, _, err := camera.ReadImage(context.Background(), ds)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.Close(context.Background()), test.ShouldBeNil)

	pCtx.GotDebugImage(processedEdges, "edges-preprocessed-aligned-depth")

	return nil
}

func TestDepthSourceGripper(t *testing.T) {
	debugVideoTransformOrSkip(t)
	proj, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(
		utils.ResolveFile("components/camera/transformpipeline/data/gripper_parameters.json"),
	)
	test.That(t, err, test.ShouldBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1/depth", "*.png", "")
	err = d.Process(t, &depthSourceTestHelper{proj})
	test.That(t, err, test.ShouldBeNil)
}

func TestDepthSourceIntel(t *testing.T) {
	debugVideoTransformOrSkip(t)
	proj, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(
		utils.ResolveFile("components/camera/transformpipeline/data/intel515_parameters.json"),
	)
	test.That(t, err, test.ShouldBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515/depth", "*.png", "")
	err = d.Process(t, &depthSourceTestHelper{proj})
	test.That(t, err, test.ShouldBeNil)
}
