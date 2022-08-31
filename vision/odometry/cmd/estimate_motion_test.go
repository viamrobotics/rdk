package main

import (
	"testing"

	"go.viam.com/rdk/rimage"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestRun(t *testing.T) {
	path1 := artifact.MustPath("vision/odometry/000001.png")
	path2 := artifact.MustPath("vision/odometry/000002.png")
	cfgPath := artifact.MustPath("vision/odometry/vo_config.json")
	// load images
	img1, err := rimage.NewImageFromFile(path1)
	test.That(t, err, test.ShouldBeNil)
	img2, err := rimage.NewImageFromFile(path2)
	test.That(t, err, test.ShouldBeNil)
	im1 := rimage.ConvertImage(img1)
	im2 := rimage.ConvertImage(img2)
	// motion
	_, motion, err := RunMotionEstimation(im1, im2, cfgPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, motion.Translation.At(0, 0), test.ShouldBeLessThan, -2.0)
	test.That(t, motion.Translation.At(1, 0), test.ShouldBeGreaterThan, 0.0)
	test.That(t, motion.Translation.At(2, 0), test.ShouldBeLessThan, -0.8)
}
