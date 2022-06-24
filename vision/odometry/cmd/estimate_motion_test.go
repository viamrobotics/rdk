package main

import (
	"github.com/edaniels/golog"
	"go.viam.com/rdk/rimage"
	"image"
	"os"
	"testing"

	"go.viam.com/test"
)

type voTestHelper struct{}

func getImageFromFilePath(filePath string) (image.Image, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	image, _, err := image.Decode(f)
	return image, err
}

func (h *voTestHelper) Process(t *testing.T, pCtx *rimage.ProcessorContext, fn string, img image.Image, logger golog.Logger) error {
	t.Helper()
	var err error
	err = run()
	test.That(t, err, test.ShouldBeNil)
	img1, err := getImageFromFilePath("/tmp/img1.png")
	test.That(t, err, test.ShouldBeNil)
	img2, err := getImageFromFilePath("/tmp/img2.png")
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(img1, "img1")
	pCtx.GotDebugImage(img2, "img2")

	return nil
}

func TestRun(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "vision/odometry", "*.png", false)
	err := d.Process(t, &voTestHelper{})
	test.That(t, err, test.ShouldBeNil)
}
