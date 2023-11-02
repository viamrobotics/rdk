package align

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func TestAlignExtrinsics(t *testing.T) {
	logger := logging.NewTestLogger(t)
	conf, err := config.Read(context.Background(), utils.ResolveFile("components/camera/align/data/extrinsics_cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := conf.FindComponent("extrinsics_cam")
	test.That(t, c, test.ShouldNotBeNil)

	extConf, ok := c.ConvertedAttributes.(*extrinsicsConfig)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, extConf, test.ShouldNotBeNil)
	img, err := rimage.NewImageFromFile(artifact.MustPath("align/intel515/chairs_color.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("align/intel515/chairs.png"))
	test.That(t, err, test.ShouldBeNil)
	// create the source cameras
	colorSrc := &videosource.StaticSource{ColorImg: img}
	colorVideoSrc, err := camera.NewVideoSourceFromReader(context.Background(), colorSrc, nil, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	depthSrc := &videosource.StaticSource{DepthImg: dm}
	depthVideoSrc, err := camera.NewVideoSourceFromReader(context.Background(), depthSrc, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)

	// create the alignment camera
	is, err := newColorDepthExtrinsics(context.Background(), colorVideoSrc, depthVideoSrc, extConf, logger)
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

	test.That(t, colorVideoSrc.Close(context.Background()), test.ShouldBeNil)
	test.That(t, depthVideoSrc.Close(context.Background()), test.ShouldBeNil)
	test.That(t, is.Close(context.Background()), test.ShouldBeNil)

	// set necessary fields to nil, expect errors
	extConf.CameraParameters = nil
	_, err = newColorDepthExtrinsics(context.Background(), colorVideoSrc, depthVideoSrc, extConf, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, transform.ErrNoIntrinsics)

	extConf.CameraParameters = &transform.PinholeCameraIntrinsics{Width: -1, Height: -1}
	_, err = newColorDepthExtrinsics(context.Background(), colorVideoSrc, depthVideoSrc, extConf, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "Got illegal dimensions")

	extConf.IntrinsicExtrinsic = nil
	_, err = newColorDepthExtrinsics(context.Background(), colorVideoSrc, depthVideoSrc, extConf, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *transform.DepthColorIntrinsicsExtrinsics")
}
