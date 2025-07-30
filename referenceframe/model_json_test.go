package referenceframe

import (
	"encoding/json"
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
		"components/arm/example_kinematics/xarm6_kinematics_test.json",
		"components/arm/example_kinematics/xarm7_kinematics_test.json",
		"referenceframe/testfiles/ur5eDH.json",
		"components/arm/example_kinematics/ur5e.json",
		"components/arm/example_kinematics/dofbot.json",
	}

	badFiles := []string{
		"referenceframe/testfiles/kinematicsloop.json",
		"referenceframe/testfiles/worldjoint.json",
		"referenceframe/testfiles/worldlink.json",
		"referenceframe/testfiles/worldDH.json",
		"referenceframe/testfiles/missinglink.json",
	}

	badFilesErrors := []error{
		ErrCircularReference,
		NewReservedWordError("link", "world"),
		NewReservedWordError("joint", "world"),
		ErrNeedOneEndEffector, // 0 end effectors
		ErrNeedOneEndEffector, // 2 end effectors
	}

	for _, f := range goodFiles {
		t.Run(f, func(tt *testing.T) {
			model, err := ParseModelJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldBeNil)

			smodel, ok := model.(*SimpleModel)
			test.That(t, ok, test.ShouldBeTrue)
			data, err := json.Marshal(smodel.modelConfig)
			test.That(t, err, test.ShouldBeNil)

			model2, err := UnmarshalModelJSON(data, "")
			test.That(t, err, test.ShouldBeNil)

			smodel2, ok := model2.(*SimpleModel)
			test.That(t, ok, test.ShouldBeTrue)

			data2, err := json.Marshal(smodel2.modelConfig)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, data, test.ShouldResemble, data2)
		})
	}

	for i, f := range badFiles {
		t.Run(f, func(tt *testing.T) {
			_, err := ParseModelJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, badFilesErrors[i].Error())
		})
	}
}
