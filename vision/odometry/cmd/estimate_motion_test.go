package main

import (
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/rimage"
)

type voTestHelper struct{}

func (h *voTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	t.Helper()
	var err error
	// Calling the function `RunMotionEstimation` which is defined in `vision/odometry/main.go`
	img1, img2, err := RunMotionEstimation()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(img1, "img1_kps")
	pCtx.GotDebugImage(img2, "img2_kps")

	return nil
}

func TestRun(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "vision/odometry", "*.png", false)
	err := d.Process(t, &voTestHelper{})
	test.That(t, err, test.ShouldBeNil)
}
