package transform

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
)

func TestRGBDToPointCloud(t *testing.T) {
	logger := logging.NewTestLogger(t)
	img, err := rimage.NewImageFromFile(artifact.MustPath("transform/align-test-1615761793_color.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("transform/align-test-1615761793.png"))
	test.That(t, err, test.ShouldBeNil)

	// from experimentation
	config := &AlignConfig{
		ColorInputSize:  image.Point{1024, 768},
		ColorWarpPoints: []image.Point{{604, 575}, {695, 115}},
		DepthInputSize:  image.Point{224, 171},
		DepthWarpPoints: []image.Point{{89, 109}, {206, 132}},
		OutputSize:      image.Point{448, 342},
		OutputOrigin:    image.Point{227, 160},
	}
	dct, err := NewDepthColorWarpTransforms(config, logger)
	test.That(t, err, test.ShouldBeNil)

	// align images first
	col, dm, err := dct.AlignColorAndDepthImage(img, dm)
	test.That(t, err, test.ShouldBeNil)
	// project
	pc, err := dct.RGBDToPointCloud(col, dm)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
	// crop
	pcCrop, err := dct.RGBDToPointCloud(col, dm, image.Rectangle{image.Point{20, 20}, image.Point{40, 40}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pcCrop.Size(), test.ShouldEqual, 400)
	// crop error
	_, err = dct.RGBDToPointCloud(col, dm, image.Rectangle{image.Point{20, 20}, image.Point{40, 40}}, image.Rectangle{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "more than one cropping rectangle")

	// image with depth with depth missing should return error
	img, err = rimage.NewImageFromFile(artifact.MustPath("transform/align-test-1615761793_color.png"))
	test.That(t, err, test.ShouldBeNil)

	pcBad, err := dct.RGBDToPointCloud(img, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, pcBad, test.ShouldBeNil)
}

func TestWarpPointsTo3D(t *testing.T) {
	logger := logging.NewTestLogger(t)
	img, err := rimage.NewImageFromFile(artifact.MustPath("transform/align-test-1615761793_color.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("transform/align-test-1615761793.png"))
	test.That(t, err, test.ShouldBeNil)

	// from experimentation
	config := &AlignConfig{
		ColorInputSize:  image.Point{1024, 768},
		ColorWarpPoints: []image.Point{{604, 575}, {695, 115}},
		DepthInputSize:  image.Point{224, 171},
		DepthWarpPoints: []image.Point{{89, 109}, {206, 132}},
		OutputSize:      image.Point{448, 342},
		OutputOrigin:    image.Point{227, 160},
	}
	testPoint := image.Point{0, 0}
	dct, err := NewDepthColorWarpTransforms(config, logger)
	test.That(t, err, test.ShouldBeNil)
	// align the images
	img, dm, err = dct.AlignColorAndDepthImage(img, dm)
	test.That(t, err, test.ShouldBeNil)
	// Check to see if the origin point on the pointcloud transformed correctly
	vec, err := dct.ImagePointTo3DPoint(config.OutputOrigin, dm.Get(config.OutputOrigin))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vec.X, test.ShouldEqual, 0.0)
	test.That(t, vec.Y, test.ShouldEqual, 0.0)
	// test out To3D
	vec, err = dct.ImagePointTo3DPoint(testPoint, dm.Get(testPoint))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vec.Z, test.ShouldEqual, float64(dm.Get(testPoint)))
	// out of bounds - panic
	testPoint = image.Point{img.Width(), img.Height()}
	test.That(t, func() { dct.ImagePointTo3DPoint(testPoint, dm.Get(testPoint)) }, test.ShouldPanic)
}
