package objectdetection

import (
	"image"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

func TestColorDetector(t *testing.T) {
	// make the original source
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)
	theColor := rimage.NewColor(79, 56, 21) // an orange color
	hue, _, _ := theColor.HsvNormal()
	// detector with error
	_, err = NewColorDetector(8., hue)
	test.That(t, err, test.ShouldBeError, errors.New("tolerance must be between 0.0 and 1.0. Got 8.00000"))
	// detector with 100% tolerance
	d, err := NewColorDetector(1., hue)
	test.That(t, err, test.ShouldBeNil)
	f := NewAreaFilter(150000)
	det, err := Build(nil, d, f)
	test.That(t, err, test.ShouldBeNil)
	result, err := det(img)
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
