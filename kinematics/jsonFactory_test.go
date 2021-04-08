package kinematics

import (
	"testing"

	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

// Tests that yml files are properly parsed and correctly loaded into the model
// Should not need to actually test the contained rotation/translation values
// since that will be caught by tests to the actual kinematics
// So we'll just check that we read in the right number of joints
func TestParseJSONFile(t *testing.T) {
	logger := golog.NewTestLogger(t)
	model, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	if len(model.Joints) != 6 {
		t.Fatalf("Incorrect number of joints loaded for wx250s")
	}
	model, err = ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s_test.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	if len(model.Joints) != 5 {
		t.Fatalf("Incorrect number of joints loaded")
	}
}
