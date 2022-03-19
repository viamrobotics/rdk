package imagesource

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/component/camera"
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

func TestUndistortImageWithDepth(t *testing.T) {
	img, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)
	source := &StaticSource{img}
	// no stream type
	us := &undistortSource{source, "", undistortTestParams}
	_, _, err = us.Next(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how to decode stream")

	// success
	us = &undistortSource{source, camera.BothStream, undistortTestParams}
	corrected, _, err := us.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	result, ok := corrected.(*rimage.ImageWithDepth)
	test.That(t, ok, test.ShouldEqual, true)

	sourceImg, _, err := source.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	expected, ok := sourceImg.(*rimage.ImageWithDepth)
	test.That(t, ok, test.ShouldEqual, true)

	outDir, err = testutils.TempDir("", "imagesource")
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/expected_color.jpg", expected.Color)
	test.That(t, err, test.ShouldBeNil)
	err = rimage.WriteImageToFile(outDir+"/result_color.jpg", result.Color)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, result.Color, test.ShouldResemble, expected.Color)
	test.That(t, result.Depth, test.ShouldResemble, expected.Depth)
	test.That(t, result.IsAligned(), test.ShouldEqual, expected.IsAligned())

	// no color channel
	img.Color = nil
	us = &undistortSource{source, camera.BothStream, undistortTestParams}
	_, _, err = us.Next(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "input image is nil")

	// no depth channel
	img, err = rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)
	source = &StaticSource{img}
	img.Depth = nil
	us = &undistortSource{source, camera.BothStream, undistortTestParams}
	_, _, err = us.Next(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "input DepthMap is nil")

	err = us.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestUndistortImage(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)
	source := &StaticSource{img}

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
}

func TestUndistortDepthMap(t *testing.T) {
	img, err := rimage.ParseDepthMap(artifact.MustPath("rimage/board1.dat.gz"))
	test.That(t, err, test.ShouldBeNil)
	source := &StaticSource{img}

	// success
	us := &undistortSource{source, camera.DepthStream, undistortTestParams}
	corrected, _, err := us.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	result, ok := corrected.(*rimage.ImageWithDepth)
	test.That(t, ok, test.ShouldEqual, true)

	sourceImg, _, err := source.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	expected, ok := sourceImg.(*rimage.DepthMap)
	test.That(t, ok, test.ShouldEqual, true)

	test.That(t, result.Depth, test.ShouldResemble, expected)
}
