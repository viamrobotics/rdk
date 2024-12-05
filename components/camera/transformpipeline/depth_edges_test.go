//go:build !no_cgo

package transformpipeline

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func TestDepthSource(t *testing.T) {
	img, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath("rimage/board1_gray_small.png"))
	test.That(t, err, test.ShouldBeNil)
	source, err := camera.NewVideoSourceFromReader(context.Background(), &fake.StaticSource{DepthImg: img}, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	am := utils.AttributeMap{
		"high_threshold": 0.85,
		"low_threshold":  0.40,
		"blur_radius":    3.0,
	}
	ds, stream, err := newDepthEdgesTransform(context.Background(), source, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)
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
	logger logging.Logger,
) error {
	t.Helper()
	dm, err := rimage.ConvertImageToDepthMap(context.Background(), img)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "aligned-depth")

	// create edge map
	source, err := camera.NewVideoSourceFromReader(context.Background(), &fake.StaticSource{DepthImg: dm}, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	am := utils.AttributeMap{
		"high_threshold": 0.85,
		"low_threshold":  0.40,
		"blur_radius":    3.0,
	}
	ds, stream, err := newDepthEdgesTransform(context.Background(), source, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)
	edges, _, err := camera.ReadImage(context.Background(), ds)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.Close(context.Background()), test.ShouldBeNil)

	pCtx.GotDebugImage(edges, "edges-aligned-depth")

	// make point cloud
	fixedPointCloud := depthadapter.ToPointCloud(dm, h.proj)
	test.That(t, fixedPointCloud.MetaData().HasColor, test.ShouldBeFalse)
	pCtx.GotDebugPointCloud(fixedPointCloud, "aligned-pointcloud")

	// preprocess depth map
	source, err = camera.NewVideoSourceFromReader(context.Background(), &fake.StaticSource{DepthImg: dm}, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	rs, stream, err := newDepthPreprocessTransform(context.Background(), source)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)

	output, _, err := camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	preprocessed, err := rimage.ConvertImageToDepthMap(context.Background(), output)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	pCtx.GotDebugImage(preprocessed.ToPrettyPicture(0, rimage.MaxDepth), "preprocessed-aligned-depth")
	preprocessedPointCloud := depthadapter.ToPointCloud(preprocessed, h.proj)
	test.That(t, preprocessedPointCloud.MetaData().HasColor, test.ShouldBeFalse)
	pCtx.GotDebugPointCloud(preprocessedPointCloud, "preprocessed-aligned-pointcloud")

	source, err = camera.NewVideoSourceFromReader(
		context.Background(),
		&fake.StaticSource{DepthImg: preprocessed},
		nil,
		camera.UnspecifiedStream,
	)
	test.That(t, err, test.ShouldBeNil)
	ds, stream, err = newDepthEdgesTransform(context.Background(), source, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)
	processedEdges, _, err := camera.ReadImage(context.Background(), ds)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.Close(context.Background()), test.ShouldBeNil)

	pCtx.GotDebugImage(processedEdges, "edges-preprocessed-aligned-depth")

	return nil
}

func TestDepthSourceGripper(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "align/gripper1/depth", "*.png", "")

	proj, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(
		utils.ResolveFile("components/camera/transformpipeline/data/gripper_parameters.json"),
	)
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &depthSourceTestHelper{proj})
	test.That(t, err, test.ShouldBeNil)
}

func TestDepthSourceIntel(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "align/intel515/depth", "*.png", "")

	proj, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(
		utils.ResolveFile("components/camera/transformpipeline/data/intel515_parameters.json"),
	)
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &depthSourceTestHelper{proj})
	test.That(t, err, test.ShouldBeNil)
}
