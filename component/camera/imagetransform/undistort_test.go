package imagetransform

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/camera/imagesource"
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
	source := &imagesource.StaticSource{ColorImg: img}
	cam, err := camera.New(source, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = cam.GetProperties(context.Background())
	test.That(t, err, test.ShouldNotBeNil)

	// no camera parameters
	attrs := &transformConfig{}
	_, err = newUndistortSource(context.Background(), cam, attrs)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)

	// bad stream type
	source = &imagesource.StaticSource{
		ColorImg: img,
		Proj:     undistortTestParams,
	}
	cam, err = camera.New(source, nil)
	test.That(t, err, test.ShouldBeNil)
	attrs.Stream = "fake"
	us, err := newUndistortSource(context.Background(), cam, attrs)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = us.Next(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how to decode stream")

	// success - attrs has camera parameters
	attrs.Stream = string(camera.ColorStream)
	us, err = newUndistortSource(context.Background(), cam, attrs)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = us.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// success - attrs does not have cam parameters, but source does
	source = &imagesource.StaticSource{ColorImg: img}
	proj, _ := camera.GetProjector(context.Background(), nil, nil)
	cam, err = camera.New(source, proj)
	test.That(t, err, test.ShouldBeNil)

	attrs.Stream = string(camera.ColorStream)
	us, err = newUndistortSource(context.Background(), cam, attrs)
	test.That(t, err, test.ShouldBeNil)
	_, _, err = us.Next(context.Background())

	err = viamutils.TryClose(context.Background(), us)
	test.That(t, err, test.ShouldBeNil)
}

func TestUndistortImage(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{ColorImg: img}

	// success
	us := &undistortSource{source, camera.ColorStream, undistortTestParams}
	corrected, _, err := us.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	result, ok := corrected.(*rimage.Image)
	test.That(t, ok, test.ShouldEqual, true)

	sourceImg, _, err := source.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	expected, ok := sourceImg.(*rimage.Image)
	test.That(t, ok, test.ShouldEqual, true)

	test.That(t, result, test.ShouldResemble, expected)

	// bad source
	source = &imagesource.StaticSource{ColorImg: rimage.NewImage(10, 10)}
	us = &undistortSource{source, camera.ColorStream, undistortTestParams}
	_, _, err = us.Next(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "img dimension and intrinsics don't match")
}

func TestUndistortDepthMap(t *testing.T) {
	img, err := rimage.ParseDepthMap(artifact.MustPath("rimage/board1.dat.gz"))
	test.That(t, err, test.ShouldBeNil)
	source := &imagesource.StaticSource{DepthImg: img}

	// success
	us := &undistortSource{source, camera.DepthStream, undistortTestParams}
	corrected, _, err := us.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	result, ok := corrected.(*rimage.DepthMap)
	test.That(t, ok, test.ShouldEqual, true)

	sourceImg, _, err := source.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	expected, ok := sourceImg.(*rimage.DepthMap)
	test.That(t, ok, test.ShouldEqual, true)

	test.That(t, result, test.ShouldResemble, expected)

	// bad source
	source = &imagesource.StaticSource{DepthImg: rimage.NewEmptyDepthMap(10, 10)}
	us = &undistortSource{source, camera.DepthStream, undistortTestParams}
	_, _, err = us.Next(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "img dimension and intrinsics don't match")

	// can't convert image to depth map
	source = &imagesource.StaticSource{ColorImg: rimage.NewImage(10, 10)}
	us = &undistortSource{source, camera.DepthStream, undistortTestParams}
	_, _, err = us.Next(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "don't know how to make DepthMap")
}
