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
		"components/arm/example_kinematics/xarm6_kinematics_test.json",
		"components/arm/example_kinematics/xarm7_kinematics_test.json",
		"referenceframe/testjson/ur5eDH.json",
		"components/arm/universalrobots/ur5e.json",
		"components/arm/fake/dofbot.json",
	}

	badFiles := []string{
		"referenceframe/testjson/kinematicsloop.json",
		"referenceframe/testjson/worldjoint.json",
		"referenceframe/testjson/worldlink.json",
		"referenceframe/testjson/worldDH.json",
		"referenceframe/testjson/missinglink.json",
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

			data, err := model.MarshalJSON()
			test.That(t, err, test.ShouldBeNil)

			model2, err := UnmarshalModelJSON(data, "")
			test.That(t, err, test.ShouldBeNil)

			data2, err := model2.MarshalJSON()
			test.That(t, err, test.ShouldBeNil)

			test.That(t, data, test.ShouldResemble, data2)
		})
	}

	for i, f := range badFiles {
		t.Run(f, func(tt *testing.T) {
			_, err := ParseModelJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldEqual, badFilesErrors[i].Error())
		})
	}
}
