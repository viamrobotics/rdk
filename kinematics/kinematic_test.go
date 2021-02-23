package kinematics

import (
	"testing"

	"github.com/edaniels/test"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/viamrobotics/robotcore/testutils"
)

// This should test all of the kinematics functions
func TestAllKinematics(t *testing.T) {
	m, err := ParseJSONFile(testutils.ResolveFile("kinematics/models/mdl/wx250s_test.json"))
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 0,-365, 360.25
	// This may change if we flip the Y axis
	m.ForwardPosition()
	mat := m.GetOperationalPosition(0).Matrix()
	m1 := mgl64.Translate3D(0, -365, 360.25)
	if !mat.ApproxEqualThreshold(m1, 0.0000001) {
		t.Fatalf("Starting 6d position incorrect")
	}

	newPos := []float64{1.1, 0.1, 1.3, 0, 0, -1}
	newExpect := mgl64.NewVecNFromData([]float64{69.80961299265694, -35.53086645234494, 674.4093770982129, -84.6760069731363, 8.222778583003974, 6.1125444468247565})
	m.SetPosition(newPos)
	m.ForwardPosition()
	new6d := mgl64.NewVecNFromData(m.Get6dPosition(0))
	if !new6d.ApproxEqual(newExpect) {
		t.Fatalf("Calculated 6d position incorrect")
	}
}
