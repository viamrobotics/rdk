package kinematics

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/utils"
)

// Tests that yml files are properly parsed and correctly loaded into the model
// Should not need to actually test the contained rotation/translation values
// since that will be caught by tests to the actual kinematics
// So we'll just check that we read in the right number of joints
func TestParseJSONFile(t *testing.T) {
	model, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(model.Joints), test.ShouldEqual, 6)
	model, err = ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_test.json"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(model.Joints), test.ShouldEqual, 5)
}
