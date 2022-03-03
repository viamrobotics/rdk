package segmentation

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

func TestSegmenterRegistry(t *testing.T) {
	fn := func(ctx context.Context, c camera.Camera, parameters config.AttributeMap) ([]*vision.Object, error) {
		return []*vision.Object{vision.NewEmptyObject()}, nil
	}
	params := struct {
		VariableOne int    `json:"int_var"`
		VariableTwo string `json:"string_var"`
	}{}
	fnName := "x"
	// no segmenter
	test.That(t, func() { RegisterSegmenter(fnName, Registration{nil, []string{}}) }, test.ShouldPanic)
	// success
	RegisterSegmenter(fnName, Registration{fn, utils.JSONTags(params)})
	// look up
	creator, err := SegmenterLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, creator.Segmenter, test.ShouldEqual, fn)
	test.That(t, creator.Parameters, test.ShouldResemble, []string{"int_var", "string_var"})
	creator, err = SegmenterLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Segmenter with name")
	test.That(t, creator, test.ShouldBeNil)
	// duplicate
	test.That(t, func() { RegisterSegmenter(fnName, Registration{fn, utils.JSONTags(params)}) }, test.ShouldPanic)
}
