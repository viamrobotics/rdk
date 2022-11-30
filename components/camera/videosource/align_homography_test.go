package videosource

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestAlignHomography(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.Read(context.Background(), utils.ResolveFile("components/camera/videosource/data/homography_cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := conf.FindComponent("homography_cam")
	test.That(t, c, test.ShouldNotBeNil)

	attrs, ok := c.ConvertedAttributes.(*alignHomographyAttrs)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, attrs, test.ShouldNotBeNil)
	img, err := rimage.NewImageFromFile(artifact.MustPath("align/intel515/chairs_color.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("align/intel515/chairs.png"))
	test.That(t, err, test.ShouldBeNil)
	// create the source cameras
	colorSrc := &StaticSource{ColorImg: img}
	colorCam, err := camera.NewFromReader(context.Background(), colorSrc, nil, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	depthSrc := &StaticSource{DepthImg: dm}
	depthCam, err := camera.NewFromReader(context.Background(), depthSrc, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)

	// create the alignment camera
	is, err := newAlignColorDepthHomography(context.Background(), colorCam, depthCam, attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	// get images and point clouds
	alignedPointCloud, err := is.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, alignedPointCloud, test.ShouldNotBeNil)
	outImage, _, err := camera.ReadImage(context.Background(), is)
	test.That(t, err, test.ShouldBeNil)
	outDepth, ok := outImage.(*rimage.DepthMap)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, outDepth, test.ShouldNotBeNil)

	test.That(t, colorCam.Close(context.Background()), test.ShouldBeNil)
	test.That(t, depthCam.Close(context.Background()), test.ShouldBeNil)
	test.That(t, is.Close(context.Background()), test.ShouldBeNil)
}
