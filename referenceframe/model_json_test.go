package referenceframe

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

// Tests that yml files are properly parsed and correctly loaded into the model
// Should not need to actually test the contained rotation/translation values
// since that will be caught by tests to the actual kinematics
// So we'll just check that we read in the right number of joints.
func TestParseJSONFile(t *testing.T) {
	goodFiles := []string{
		"component/arm/trossen/wx250s_kinematics.json",
		"component/arm/trossen/wx250s_test.json",
		"component/arm/universalrobots/ur5e_DH.json",
		"component/arm/varm/v1.json",
	}

	badFiles := []string{
		"kinematics/testjson/kinematicsloop.json",
		"kinematics/testjson/worldjoint.json",
		"kinematics/testjson/worldlink.json",
		"kinematics/testjson/worldDH.json",
	}

	for _, f := range goodFiles {
		t.Run(f, func(tt *testing.T) {
			model, err := ParseModelJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldBeNil)

			data, err := model.MarshalJSON()
			test.That(t, err, test.ShouldBeNil)

			model2, err := UnmarshalModelJSON(data, "")
			test.That(t, err, test.ShouldBeNil)

			test.That(t, model.AlmostEquals(model2), test.ShouldBeTrue)
		})
	}

	for _, f := range badFiles {
		t.Run(f, func(tt *testing.T) {
			_, err := ParseModelJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldNotBeNil)
		})
	}
}
