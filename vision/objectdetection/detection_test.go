package objectdetection

import (
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/rimage"
)

func TestBuildFunc(t *testing.T) {
	img := rimage.NewImage(400, 400)
	_, err := Build(nil, nil, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "must have a Detector")
	// make simple detector
	det := func(image.Image) ([]Detection, error) {
		return []Detection{&detection2D{}}, nil
	}
	pipeline, err := Build(nil, det, nil)
	test.That(t, err, test.ShouldBeNil)
	res, err := pipeline(img)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldHaveLength, 1)
	// make simple filter
	filt := func(d []Detection) []Detection {
		return []Detection{}
	}
	pipeline, err = Build(nil, det, filt)
	test.That(t, err, test.ShouldBeNil)
	res, err = pipeline(img)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldHaveLength, 0)
}
