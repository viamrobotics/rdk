package referenceframe

import (
	"encoding/json"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func TestLimitsParsing(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	smodel, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	// Set custom limits on the simple model. Such that when we roundtrip through JSON
	// serialization, we can see those values persist.
	smodel.limits[0].Min = 0
	smodel.limits[0].Max = 1

	data, err := smodel.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	simpleModelDeserialized := new(SimpleModel)
	err = simpleModelDeserialized.UnmarshalJSON(data)
	test.That(t, err, test.ShouldBeNil)

	// Assert that the min/max member values reflect the above assignments. As well as the result of
	// the interface `DoF` method.
	test.That(t, simpleModelDeserialized.limits[0].Min, test.ShouldEqual, 0)
	test.That(t, simpleModelDeserialized.limits[0].Max, test.ShouldEqual, 1)
	test.That(t, simpleModelDeserialized.DoF()[0], test.ShouldResemble, Limit{0, 1})
}

// Tests that yml files are properly parsed and correctly loaded into the model
// Should not need to actually test the contained rotation/translation values
// since that will be caught by tests to the actual kinematics
// So we'll just check that we read in the right number of joints.
func TestParseJSONFile(t *testing.T) {
	goodFiles := []string{
		"components/arm/fake/kinematics/xarm6.json",
		"components/arm/fake/kinematics/xarm7.json",
		"referenceframe/testfiles/ur5eDH.json",
		"components/arm/fake/kinematics/ur5e.json",
		"components/arm/fake/kinematics/dofbot.json",
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
