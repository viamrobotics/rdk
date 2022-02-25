package objectdetection

import (
	"image"
	"testing"

	"go.viam.com/test"
)

func TestBuildFunc(t *testing.T) {
	_, err := Build(nil, nil, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "must have a Detector")
	det := func(image.Image) ([]Detection, error) {
		return []Detection{&detection2D{}}, nil
	}
	res, err := Build(nil, det, nil)

}
