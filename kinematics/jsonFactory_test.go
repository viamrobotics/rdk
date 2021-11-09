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
	_, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	_, err = ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_test.json"), "")
	test.That(t, err, test.ShouldBeNil)

	_, err = ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e_DH.json"), "")
	test.That(t, err, test.ShouldBeNil)

	_, err = ParseJSONFile(utils.ResolveFile("kinematics/testjson/kinematicsloop.json"), "")
	test.That(t, err, test.ShouldNotBeNil)

	_, err = ParseJSONFile(utils.ResolveFile("kinematics/testjson/worldjoint.json"), "")
	test.That(t, err, test.ShouldNotBeNil)

	_, err = ParseJSONFile(utils.ResolveFile("kinematics/testjson/worldlink.json"), "")
	test.That(t, err, test.ShouldNotBeNil)

	_, err = ParseJSONFile(utils.ResolveFile("kinematics/testjson/worldDH.json"), "")
	test.That(t, err, test.ShouldNotBeNil)

}
