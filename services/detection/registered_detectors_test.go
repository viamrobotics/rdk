package detection

import (
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

func TestDetectorRegistry(t *testing.T) {
	fn := func(image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{objdet.NewDetection(image.Rectangle{}, 0.0, "")}, nil
	}
	params := struct {
		VariableOne int    `json:"int_var"`
		VariableTwo string `json:"string_var"`
	}{}
	fnName := "x"
	// no detector
	test.That(t, func() { RegisterDetector(fnName, DetectorRegistration{nil, []utils.TypedName{}}) }, test.ShouldPanic)
	// success
	RegisterDetector(fnName, DetectorRegistration{fn, utils.JSONTags(params)})
	// detector names
	names := DetectorNames()
	test.That(t, names, test.ShouldNotBeNil)
	test.That(t, names, test.ShouldContain, fnName)
	// look up
	creator, err := DetectorLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, creator.Detector, test.ShouldEqual, fn)
	test.That(t, creator.Parameters, test.ShouldResemble, []utils.TypedName{{"int_var", "int"}, {"string_var", "string"}})
	creator, err = DetectorLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Detector with name")
	test.That(t, creator, test.ShouldBeNil)
	// duplicate
	test.That(t, func() { RegisterDetector(fnName, DetectorRegistration{fn, utils.JSONTags(params)}) }, test.ShouldPanic)
}
