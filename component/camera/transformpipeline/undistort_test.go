package transformpipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/camera/videosource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
)

// not the real intrinsic parameters of the image, only for testing purposes.
var undistortTestParams = &transform.PinholeCameraIntrinsics{
	Width:  1280,
	Height: 720,
	Fx:     1.,
	Fy:     1.,
	Ppx:    0.,
	Ppy:    0.,
	Distortion: transform.DistortionModel{
		RadialK1:     0.,
		RadialK2:     0.,
		TangentialP1: 0.,
		TangentialP2: 0.,
		RadialK3:     0.,
	},
}

func TestUndistortSetup(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)
	source := gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})

	// no camera parameters
	am := config.AttributeMap{}
	_, err = newUndistortTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)

	// bad stream type
	source = gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	am = config.AttributeMap{"camera_parameters": undistortTestParams}
	us, err := newUndistortTransform(context.Background(), source, camera.StreamType("fake"), am)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = camera.ReadImage(context.Background(), us)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how to decode stream")
	test.That(t, us.Close(context.Background()), test.ShouldBeNil)

	// success - attrs has camera parameters
	us, err = newUndistortTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = camera.ReadImage(context.Background(), us)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, us.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)
}

func TestUndistortImage(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)
	source := gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})

	// success
	am := config.AttributeMap{"camera_parameters": undistortTestParams}
	us, err := newUndistortTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	corrected, _, err := camera.ReadImage(context.Background(), us)
	test.That(t, err, test.ShouldBeNil)
	result, ok := corrected.(*rimage.Image)
	test.That(t, ok, test.ShouldEqual, true)
	test.That(t, us.Close(context.Background()), test.ShouldBeNil)

	sourceImg, _, err := camera.ReadImage(context.Background(), source)
	test.That(t, err, test.ShouldBeNil)
	expected, ok := sourceImg.(*rimage.Image)
	test.That(t, ok, test.ShouldEqual, true)

	test.That(t, result, test.ShouldResemble, expected)

	// bad source
	source = gostream.NewVideoSource(&videosource.StaticSource{ColorImg: rimage.NewImage(10, 10)}, prop.Video{})
	us, err = newUndistortTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = camera.ReadImage(context.Background(), us)
	test.That(t, err.Error(), test.ShouldContainSubstring, "img dimension and intrinsics don't match")
	test.That(t, us.Close(context.Background()), test.ShouldBeNil)
}

func TestUndistortDepthMap(t *testing.T) {
	img, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1_gray.png"))
	test.That(t, err, test.ShouldBeNil)
	source := gostream.NewVideoSource(&videosource.StaticSource{DepthImg: img}, prop.Video{})

	// success
	am := config.AttributeMap{"camera_parameters": undistortTestParams}
	us, err := newUndistortTransform(context.Background(), source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	corrected, _, err := camera.ReadImage(context.Background(), us)
	test.That(t, err, test.ShouldBeNil)
	result, ok := corrected.(*rimage.DepthMap)
	test.That(t, ok, test.ShouldEqual, true)
	test.That(t, us.Close(context.Background()), test.ShouldBeNil)

	sourceImg, _, err := camera.ReadImage(context.Background(), source)
	test.That(t, err, test.ShouldBeNil)
	expected, ok := sourceImg.(*rimage.DepthMap)
	test.That(t, ok, test.ShouldEqual, true)

	test.That(t, result, test.ShouldResemble, expected)

	// bad source
	source = gostream.NewVideoSource(&videosource.StaticSource{DepthImg: rimage.NewEmptyDepthMap(10, 10)}, prop.Video{})
	us, err = newUndistortTransform(context.Background(), source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = camera.ReadImage(context.Background(), us)
	test.That(t, err.Error(), test.ShouldContainSubstring, "img dimension and intrinsics don't match")
	test.That(t, us.Close(context.Background()), test.ShouldBeNil)

	// can't convert image to depth map
	source = gostream.NewVideoSource(&videosource.StaticSource{ColorImg: rimage.NewImage(10, 10)}, prop.Video{})
	us, err = newUndistortTransform(context.Background(), source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = camera.ReadImage(context.Background(), us)
	test.That(t, err.Error(), test.ShouldContainSubstring, "don't know how to make DepthMap")
	test.That(t, us.Close(context.Background()), test.ShouldBeNil)
}
