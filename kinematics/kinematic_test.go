package kinematics

import (
	"testing"

	"github.com/edaniels/test"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/viamrobotics/robotcore/testutils"
)

// This should test all of the kinematics functions
func TestAllKinematics(t *testing.T) {
	m, err := ParseJsonFile(testutils.ResolveFile("kinematics/models/mdl/wx250s_test.json"))
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 0,-365, 360.25
	// This may change if we flip the Y axis
	m.ForwardPosition()
	mat := m.GetOperationalPosition(0).Matrix()
	m1 := mgl64.Translate3D(0, -365, 360.25)
	if !mat.ApproxEqualThreshold(m1, 0.0000001) {
		t.Fatalf("Starting 6d position incorrect")
	}

}
