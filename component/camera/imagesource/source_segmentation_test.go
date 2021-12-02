package imagesource

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/core/config"
	"go.viam.com/core/rimage"
	"go.viam.com/core/utils"
	"go.viam.com/core/vision/segmentation"
)

type segmentationSourceTestHelper struct {
	attrs  config.AttributeMap
	config segmentation.ObjectConfig
}

func (h *segmentationSourceTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {

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
	fixed.SetCameraSystem(dc.projectionCamera)

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
	config, err := config.Read(utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front")
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/aligned_intel", "*.both.gz", true)
	cfg := segmentation.ObjectConfig{50000, 500, 10.}
	err = d.Process(t, &segmentationSourceTestHelper{c.Attributes, cfg})
	test.That(t, err, test.ShouldBeNil)
}
