package main

import (
	"github.com/edaniels/golog"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/odometry"
)

var logger = golog.NewLogger("visual-odometry")

func main() {
	// load cfg
	cfg := odometry.LoadMotionEstimationConfig(artifact.MustPath("vision/odometry/vo_config.json"))
	// load images
	im1, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000001.png"))
	if err != nil {
		logger.Fatal(err.Error())
	}

	im2, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000002.png"))
	if err != nil {
		logger.Fatal(err.Error())
	}
	// Estimate motion
	motion, err := odometry.EstimateMotionFrom2Frames(im1, im2, cfg)
	if err != nil {
		logger.Fatal(err.Error())
	}
	logger.Info(motion.Rotation)
	logger.Info(motion.Translation)
}
