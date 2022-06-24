package main

import (
	"github.com/edaniels/golog"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/odometry"
	"go.viam.com/utils/artifact"
)

var logger = golog.NewLogger("visual-odometry")

func run() error {
	// load cfg
	cfg := odometry.LoadMotionEstimationConfig(artifact.MustPath("vision/odometry/vo_config.json"))
	// load images
	im1, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000001.png"))
	if err != nil {
		return err
	}

	im2, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000002.png"))
	if err != nil {
		return err
	}
	// Estimate motion
	motion, err := odometry.EstimateMotionFrom2Frames(im1, im2, cfg, true)
	if err != nil {
		return err
	}
	logger.Info(motion.Rotation)
	logger.Info(motion.Translation)
	return nil
}

func main() {
	err := run()
	if err != nil {
		logger.Fatal(err.Error())
	}
}
