package main

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestRun(t *testing.T) {
	path1 := artifact.MustPath("vision/odometry/000001.png")
	path2 := artifact.MustPath("vision/odometry/000002.png")
	cfgPath := artifact.MustPath("vision/odometry/vo_config.json")
	_, _, motion, err := RunMotionEstimation(path1, path2, cfgPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, motion.Translation.At(0, 0), test.ShouldBeLessThan, -2.0)
	test.That(t, motion.Translation.At(1, 0), test.ShouldBeGreaterThan, 0.0)
	test.That(t, motion.Translation.At(2, 0), test.ShouldBeLessThan, -1.0)
}
