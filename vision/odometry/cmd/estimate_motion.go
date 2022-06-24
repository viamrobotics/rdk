package main

import (
	"image"

	"github.com/edaniels/golog"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/odometry"
)

var logger = golog.NewLogger("visual-odometry")

// RunMotionEstimation runs motion estimation between the two frames in artifacts.
func RunMotionEstimation() (image.Image, image.Image, error) {
	// load cfg
	cfg := odometry.LoadMotionEstimationConfig(artifact.MustPath("vision/odometry/vo_config.json"))
	// load images
	im1, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000001.png"))
	if err != nil {
		return nil, nil, err
	}
	im2, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000002.png"))
	if err != nil {
		return nil, nil, err
	}
	// Estimate motion
	motion, err := odometry.EstimateMotionFrom2Frames(im1, im2, cfg, true)
	if err != nil {
		return nil, nil, err
	}
	logger.Info(motion.Rotation)
	logger.Info(motion.Translation)
	img1Out, err := rimage.NewImageFromFile("/tmp/img1.png")
	if err != nil {
		return nil, nil, err
	}
	img2Out, err := rimage.NewImageFromFile("/tmp/img2.png")
	if err != nil {
		return nil, nil, err
	}
	return img1Out, img2Out, nil
}

func main() {
	if _, _, err := RunMotionEstimation(); err != nil {
		logger.Fatal(err.Error())
	}
}
