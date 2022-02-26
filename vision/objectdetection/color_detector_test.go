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
	theColor := rimage.NewColor(79, 56, 21) // a yellow color
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
