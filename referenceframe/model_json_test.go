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
		"components/arm/eva/eva_kinematics.json",
		"components/arm/xarm/xarm6_kinematics.json",
		"components/arm/xarm/xarm7_kinematics.json",
		"referenceframe/testjson/ur5eDH.json",
		"components/arm/universalrobots/ur5e.json",
		"components/arm/yahboom/dofbot.json",
	}

	badFiles := []string{
		"referenceframe/testjson/kinematicsloop.json",
		"referenceframe/testjson/worldjoint.json",
		"referenceframe/testjson/worldlink.json",
		"referenceframe/testjson/worldDH.json",
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
