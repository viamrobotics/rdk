package kinematics

import (
	"math"
	"testing"
	//~ "fmt"

	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

// This should test forward kinematics functions
func TestForwardKinematics(t *testing.T) {
	// Test fake 5DOF arm
	logger := golog.NewTestLogger(t)
	m, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s_test.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 300, 0, 360.25
	m.ForwardPosition()
	expect := []float64{300, 0, 360.25, 0, 0, 0}
	actual := m.Get6dPosition(0)

	//~ fmt.Println(actual)

	if floatDelta(expect, actual) > 0.00001 {
		t.Fatalf("Starting 6d position incorrect")
	}

	// Test the 6dof arm we actually have
	m, err = ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 365, 0, 360.25
	m.ForwardPosition()
	expect = []float64{365, 0, 360.25, 0, 0, 0}
	actual = m.Get6dPosition(0)

	if floatDelta(expect, actual) > 0.00001 {
		t.Fatalf("Starting 6d position incorrect")
	}

	//~ newPos := []float64{0.7854, -0.7854, 0, 0, 0, 0}
	//~ m.SetPosition(newPos)
	//~ m.ForwardPosition()
	//~ actual = m.Get6dPosition(0)

	//~ expect = []float64{57.5, 57.5, 545.1208197765168, 0, -45, 45}
	//~ if floatDelta(expect, actual) > 0.01 {
	//~ t.Fatalf("rotation 1 incorrect")
	//~ }
	//~ newPos = []float64{-0.7854, 0, 0, 0, 0, 0.7854}
	//~ m.SetPosition(newPos)
	//~ m.ForwardPosition()
	//~ actual = m.Get6dPosition(0)

	//~ expect = []float64{258.0935, -258.0935, 360.25, 45, 0, -45}
	//~ if floatDelta(expect, actual) > 0.01 {
	//~ t.Fatalf("rotation 2 incorrect")
	//~ }
}

func floatDelta(l1, l2 []float64) float64 {
	delta := 0.0
	for i, v := range l1 {
		delta += math.Abs(v - l2[i])
	}
	return delta
}

func TestJacobian(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s_test.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	newPos := []float64{0, 0, 0, 0, 0}
	//~ newPos := []float64{0, 0, 0, 1.5708, 0}
	m.SetPosition(newPos)
	m.ForwardPosition()

	//~ actual := m.Get6dPosition(0)
	//~ fmt.Println("test actual start", actual)

	//~ m.CalculateJacobian()

	//~ j := m.GetJacobian()

	//~ j2 := mat.NewDense(5,8, j.Raw())
	//~ fc := mat.Formatted(j2, mat.Prefix("      "), mat.Squeeze())
	//~ fmt.Printf("jac = %v", fc)
	//~ fmt.Println("")
}
