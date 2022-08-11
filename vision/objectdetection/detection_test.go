package objectdetection

import (
	"context"
	"image"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/rimage"
)

func TestBuildFunc(t *testing.T) {
	img := rimage.NewImage(400, 400)
	_, err := Build(nil, nil, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "must have a Detector")
	// detector that creates an error
	det := func(context.Context, image.Image) ([]Detection, error) {
		return nil, errors.New("detector error")
	}
	ctx := context.Background()
	pipeline, err := Build(nil, det, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = pipeline(ctx, img)
	test.That(t, err.Error(), test.ShouldEqual, "detector error")
	// make simple detector
	det = func(context.Context, image.Image) ([]Detection, error) {
		return []Detection{&detection2D{}}, nil
	}
	pipeline, err = Build(nil, det, nil)
	test.That(t, err, test.ShouldBeNil)
	res, err := pipeline(ctx, img)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldHaveLength, 1)
	// make simple filter
	filt := func(d []Detection) []Detection {
		return []Detection{}
	}
	pipeline, err = Build(nil, det, filt)
	test.That(t, err, test.ShouldBeNil)
	res, err = pipeline(ctx, img)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldHaveLength, 0)
}
