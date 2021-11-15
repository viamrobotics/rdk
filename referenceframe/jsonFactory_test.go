package referenceframe

import (
	"fmt"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/utils"
)

// Tests that yml files are properly parsed and correctly loaded into the model
// Should not need to actually test the contained rotation/translation values
// since that will be caught by tests to the actual kinematics
// So we'll just check that we read in the right number of joints
func TestParseJSONFile(t *testing.T) {
	goodFiles := []string{
		"robots/wx250s/wx250s_kinematics.json",
		"robots/wx250s/wx250s_test.json",
		"robots/universalrobots/ur5e_DH.json",
		"robots/varm/v1.json",
	}

	badFiles := []string{
		"kinematics/testjson/kinematicsloop.json",
		"kinematics/testjson/worldjoint.json",
		"kinematics/testjson/worldlink.json",
		"kinematics/testjson/worldDH.json",
	}

	for _, f := range goodFiles {
		t.Run(f, func(tt *testing.T) {
			model, err := ParseJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldBeNil)

			data, err := model.MarshalJSON()
			test.That(t, err, test.ShouldBeNil)

			model2, err := ParseJSON(data, "")
			test.That(t, err, test.ShouldBeNil)

			fmt.Printf("%#v\n%#v\n", model, model2)

			test.That(t, model.AlmostEquals(model2), test.ShouldBeTrue)
		})
	}

	for _, f := range badFiles {
		t.Run(f, func(tt *testing.T) {
			_, err := ParseJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldNotBeNil)
		})
	}

}
