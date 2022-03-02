package segmentation

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/vision"
)

func TestSegmenterRegistry(t *testing.T) {
	fn := func(ctx context.Context, c camera.Camera, parameters config.AttributeMap) ([]*vision.Object, error) {
		return []*vision.Object{vision.NewEmptyObject()}, nil
	}
	fnName := "x"
	// no segmenter
	test.That(t, func() { RegisterSegmenter(fnName, nil) }, test.ShouldPanic)
	// success
	RegisterSegmenter(fnName, fn)
	// look up
	creator, err := SegmenterLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, creator, test.ShouldEqual, fn)
	creator, err = SegmenterLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Segmenter with name")
	test.That(t, creator, test.ShouldBeNil)
	// duplicate
	test.That(t, func() { RegisterSegmenter(fnName, fn) }, test.ShouldPanic)
}
