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
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
)

func TestSegmentationSource(t *testing.T) {
	img, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)
	cameraMatrices, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(
		utils.ResolveFile("robots/configs/intel515_parameters.json"),
	)
	test.That(t, err, test.ShouldBeNil)
	img.SetProjector(cameraMatrices)
	source := &staticSource{img}
	cfg := segmentation.ObjectConfig{50000, 500, 10.}

	cs := &colorSegmentsSource{source, cfg}
	_, _, err = cs.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

type segmentationSourceTestHelper struct {
	attrs  rimage.AttrConfig
	config segmentation.ObjectConfig
}

func (h *segmentationSourceTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	ii := rimage.ConvertToImageWithDepth(img)
	// align the images
	is, err := NewDepthComposed(nil, nil, &h.attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	dc, ok := is.(*depthComposed)
	test.That(t, ok, test.ShouldBeTrue)
	fixed, err := dc.alignmentCamera.AlignImageWithDepth(ii)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "aligned-depth")

	// change to use projection camera
	fixed.SetProjector(dc.projectionCamera)

	//
	source := &staticSource{fixed}
	cs := &colorSegmentsSource{source, h.config}
	segments, _, err := cs.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(segments, "segmented-image")

	// make point cloud
	fixedPointCloud, err := fixed.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(fixedPointCloud, "aligned-pointcloud")

	// segments point cloud
	iwdSegments := rimage.ConvertToImageWithDepth(segments)
	segmentedPointCloud, err := iwdSegments.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(segmentedPointCloud, "segmented-pointcloud")

	return nil
}

func TestSegmentationSourceIntel(t *testing.T) {
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front").ConvertedAttributes.(*rimage.AttrConfig)
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/aligned_intel", "*.both.gz", true)
	cfg := segmentation.ObjectConfig{50000, 500, 10.}
	err = d.Process(t, &segmentationSourceTestHelper{*c, cfg})
	test.That(t, err, test.ShouldBeNil)
}
