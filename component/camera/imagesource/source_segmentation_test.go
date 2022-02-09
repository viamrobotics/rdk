package imagesource

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func TestSegmentationSource(t *testing.T) {
	img, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)
	cameraMatrices, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(
		utils.ResolveFile("robots/configs/intel515_parameters.json"),
	)
	test.That(t, err, test.ShouldBeNil)
	source := &staticSource{img}
	cam, err := camera.New(source, nil, nil)
	test.That(t, err, test.ShouldBeNil)

	cfg := &camera.AttrConfig{
		PlaneSize:        50000,
		SegmentSize:      500,
		ClusterRadius:    10.,
		CameraParameters: &cameraMatrices.ColorCamera,
	}
	cs, err := newColorSegmentsSource(cam, cfg)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = cs.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

type segmentationSourceTestHelper struct {
	attrs *camera.AttrConfig
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
	var fixed *rimage.ImageWithDepth
	var err error
	aligner, err := getAligner(h.attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	if !ii.IsAligned() {
		fixed, err = aligner.AlignColorAndDepthImage(ii.Color, ii.Depth)
		test.That(t, err, test.ShouldBeNil)
	} else {
		fixed = ii
	}
	pCtx.GotDebugImage(fixed.Depth.ToPrettyPicture(0, rimage.MaxDepth), "aligned-depth")

	source := &staticSource{fixed}
	cam, err := camera.New(source, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	cs, err := newColorSegmentsSource(cam, h.attrs)
	test.That(t, err, test.ShouldBeNil)
	segments, _, err := cs.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(segments, "segmented-image")

	// make point cloud
	fixedPointCloud, err := h.attrs.CameraParameters.ImageWithDepthToPointCloud(fixed)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(fixedPointCloud, "aligned-pointcloud")

	// segments point cloud
	iwdSegments := rimage.ConvertToImageWithDepth(segments)
	segmentedPointCloud, err := h.attrs.CameraParameters.ImageWithDepthToPointCloud(iwdSegments)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(segmentedPointCloud, "segmented-pointcloud")

	return nil
}

func TestSegmentationSourceIntel(t *testing.T) {
	debugImageSourceOrSkip(t)
	config, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/intel.json"))
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front").ConvertedAttributes.(*camera.AttrConfig)
	test.That(t, c, test.ShouldNotBeNil)

	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/aligned_intel", "*.both.gz", true)
	c.PlaneSize = 50000
	c.SegmentSize = 500
	c.ClusterRadius = 10.
	err = d.Process(t, &segmentationSourceTestHelper{c})
	test.That(t, err, test.ShouldBeNil)
}
