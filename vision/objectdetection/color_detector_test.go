package objectdetection

import (
	"context"
	"image"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

const colorHexString = "#4F3815" // an orange color

func TestColorDetector(t *testing.T) {
	// make the original source
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)
	ctx := context.Background()
	// detector with error
	cfg := &ColorDetectorConfig{
		SegmentSize:       150000,
		HueTolerance:      8.0,
		DetectColorString: colorHexString,
	}
	_, err = NewColorDetector(cfg)
	test.That(t, err.Error(), test.ShouldContainSubstring, "hue_tolerance_pct is required")

	cfg.HueTolerance = 1.
	cfg.SaturationCutoff = 8
	_, err = NewColorDetector(cfg)
	test.That(t, err, test.ShouldBeError, errors.New("saturation_cutoff_pct must be between 0.0 and 1.0. Got 8.00000"))

	cfg.SaturationCutoff = 1.
	cfg.ValueCutoff = 8
	_, err = NewColorDetector(cfg)
	test.That(t, err, test.ShouldBeError, errors.New("value_cutoff_pct must be between 0.0 and 1.0. Got 8.00000"))

	cfg.ValueCutoff = 1.
	_, err = NewColorDetector(cfg)
	test.That(t, err.Error(), test.ShouldContainSubstring, "too unsaturated")

	cfg.SaturationCutoff = 0.2
	_, err = NewColorDetector(cfg)
	test.That(t, err.Error(), test.ShouldContainSubstring, "too dark")

	cfg.ValueCutoff = 0.3
	cfg.DetectColorString = "#000000" // black
	_, err = NewColorDetector(cfg)
	test.That(t, err.Error(), test.ShouldContainSubstring,
		"the chosen color to detect has a saturation of 0",
	)

	cfg.DetectColorString = colorHexString
	det, err := NewColorDetector(cfg)
	test.That(t, err, test.ShouldBeNil)
	result, err := det(ctx, img)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 1)
	test.That(t, result[0].BoundingBox().Min, test.ShouldResemble, image.Point{0, 336})

	// Try with default cutoff values
	cfg.SaturationCutoff = 0
	cfg.ValueCutoff = 0
	det, err = NewColorDetector(cfg)
	test.That(t, err, test.ShouldBeNil)
	result, err = det(ctx, img)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldHaveLength, 1)
	test.That(t, result[0].BoundingBox().Min, test.ShouldResemble, image.Point{0, 336})
}

func TestHueToString(t *testing.T) {
	theColor := rimage.Red
	hue, _, _ := theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "red")
	theColor = rimage.NewColor(255, 125, 0) // orange
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "orange")
	theColor = rimage.Yellow
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "yellow")
	theColor = rimage.NewColor(125, 255, 0) // lime green
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "lime-green")
	theColor = rimage.Green
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "green")
	theColor = rimage.NewColor(0, 255, 125) // green-blue
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "green-blue")
	theColor = rimage.Cyan
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "cyan")
	theColor = rimage.NewColor(0, 125, 255) // light-blue
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "light-blue")
	theColor = rimage.Blue
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "blue")
	theColor = rimage.NewColor(125, 0, 255) // violet
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "violet")
	theColor = rimage.NewColor(255, 0, 255) // magenta
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "magenta")
	theColor = rimage.NewColor(255, 0, 125) // rose
	hue, _, _ = theColor.HsvNormal()
	test.That(t, hueToString(hue), test.ShouldEqual, "rose")
}
