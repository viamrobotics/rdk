package registry

import (
	"context"
	"testing"

	"go.viam.com/test"
)

func TestSegmenterRegistry(t *testing.T) {
	fn := func(ctx context.Context) (interface{}, error) { return 1, nil }
	fnName := "x"
	// no constructor
	test.That(t, func() { RegisterSegmenter(fnName, Segmenter{}) }, test.ShouldPanic)
	// success
	RegisterSegmenter(fnName, Segmenter{Constructor: fn})
	// look up
	creator := SegmenterLookup(fnName)
	test.That(t, creator, test.ShouldNotBeNil)
	test.That(t, creator.Constructor, test.ShouldEqual, fn)
	creator = SegmenterLookup("z")
	test.That(t, creator, test.ShouldBeNil)
	// duplicate
	test.That(t, func() { RegisterSegmenter(fnName, Segmenter{Constructor: fn}) }, test.ShouldPanic)
}
