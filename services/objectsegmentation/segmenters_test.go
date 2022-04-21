package objectsegmentation

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

func TestSegmenterMap(t *testing.T) {
	fn := func(ctx context.Context, c camera.Camera, parameters config.AttributeMap) ([]*vision.Object, error) {
		return []*vision.Object{vision.NewEmptyObject()}, nil
	}
	params := struct {
		VariableOne int    `json:"int_var"`
		VariableTwo string `json:"string_var"`
	}{}
	fnName := "x"
	segMap := make(segmenterMap)
	// no segmenter
	err := segMap.registerSegmenter(fnName, SegmenterRegistration{nil, []utils.TypedName{}})
	test.That(t, err, test.ShouldNotBeNil)
	// success
	err = segMap.registerSegmenter(fnName, SegmenterRegistration{fn, utils.JSONTags(params)})
	test.That(t, err, test.ShouldBeNil)
	// segmenter names
	names := segMap.segmenterNames()
	test.That(t, names, test.ShouldNotBeNil)
	test.That(t, names, test.ShouldContain, fnName)
	// look up
	creator, err := segMap.segmenterLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, creator.Segmenter, test.ShouldEqual, fn)
	test.That(t, creator.Parameters, test.ShouldResemble, []utils.TypedName{{"int_var", "int"}, {"string_var", "string"}})
	creator, err = segMap.segmenterLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Segmenter with name")
	test.That(t, creator, test.ShouldBeNil)
	// duplicate
	err = segMap.registerSegmenter(fnName, SegmenterRegistration{fn, utils.JSONTags(params)})
	test.That(t, err, test.ShouldNotBeNil)
}
