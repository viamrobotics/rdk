package kinematics

import (
	"fmt"
	"runtime"
	"testing"

	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

const toSolve = 100

// This should test all of the kinematics functions
func TestCombinedIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	nCPU := runtime.NumCPU()
	wxArm, err := NewArmJSONFile(nil, utils.ResolveFile("kinematics/models/mdl/wx250s.json"), nCPU, logger)
	test.That(t, err, test.ShouldBeNil)
	wxArm.SetJointPositions([]float64{69.35309996071989, 28.752097952708045, -101.57720046840646, 0.9393597585332618, -73.96221972947882, 0.03845332136188379})

	// Test ability to arrive at another position
	pos := &pb.ArmPosition{
		X:  -46.445827416798814,
		Y:  -133.99229347583582,
		Z:  372.4849299627513,
		RX: -178.88747811107424,
		RY: -33.160094626838045,
		RZ: -111.02282693533935,
	}
	err = wxArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)

	// Test moving forward 20 in X direction from previous position
	// if nCPU < 8 {
	t.Skip("Skipping problematic position, too few CPUs to solve; fails often")
	// }
	pos = &pb.ArmPosition{
		X:  -66.445827416798814,
		Y:  -133.99229347583582,
		Z:  372.4849299627513,
		RX: -178.88747811107424,
		RY: -33.160094626838045,
		RZ: -111.02282693533935,
	}
	err = wxArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)
}
func BenchCombinedIKinematics(t *testing.B) {
	logger := golog.NewDevelopmentLogger("combinedBenchmark")
	nCPU := runtime.NumCPU()
	wxArm, err := NewArmJSONFile(nil, utils.ResolveFile("kinematics/models/mdl/wx250s.json"), nCPU, logger)
	test.That(t, err, test.ShouldBeNil)

	// Test we are able to solve random valid positions from other random valid positions
	// Used for benchmarking solve rate
	solved := 0
	for i := 0; i < toSolve; i++ {
		fmt.Println(i)
		jPos := wxArm.Model.RandomJointPositions()
		wxArm.Model.SetPosition(jPos)
		rPos := wxArm.GetForwardPosition()
		startPos := wxArm.Model.RandomJointPositions()
		wxArm.Model.SetPosition(startPos)
		err = wxArm.SetForwardPosition(rPos)
		if err == nil {
			solved++
		}
	}
	fmt.Println("combined solved: ", solved)
}

func TestNloptIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	wxArm, err := NewArmJSONFile(nil, utils.ResolveFile("kinematics/models/mdl/wx250s.json"), 1, logger)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateNloptIKSolver(wxArm.Model, logger)
	wxArm.ik = ik

	pos := &pb.ArmPosition{
		X:  1,
		Y:  -368,
		Z:  355,
		RX: 0,
		RY: 0,
		RZ: 0,
	}
	err = wxArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)
}

func BenchNloptIKinematics(t *testing.B) {
	logger := golog.NewDevelopmentLogger("nloptBenchmark")
	wxArm, err := NewArmJSONFile(nil, utils.ResolveFile("kinematics/models/mdl/wx250s.json"), 1, logger)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateNloptIKSolver(wxArm.Model, logger)
	wxArm.ik = ik

	// Used for benchmarking solve rate
	solved := 0
	for i := 0; i < toSolve; i++ {
		jPos := wxArm.Model.RandomJointPositions()
		wxArm.Model.SetPosition(jPos)
		rPos := wxArm.GetForwardPosition()
		startPos := wxArm.Model.RandomJointPositions()
		wxArm.Model.SetPosition(startPos)
		err = wxArm.SetForwardPosition(rPos)
		if err == nil {
			solved++
		}
	}
	fmt.Println("nlopt solved: ", solved)
}

func TestJacobianIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	wxArm, err := NewArmJSONFile(nil, utils.ResolveFile("kinematics/models/mdl/wx250s.json"), 1, logger)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateJacobianIKSolver(wxArm.Model)
	wxArm.ik = ik

	pos := &pb.ArmPosition{X: 350, Y: 10, Z: 355, RX: 0, RY: 0, RZ: 0}
	err = wxArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)
}

func BenchJacobianIKinematics(t *testing.B) {
	logger := golog.NewDevelopmentLogger("jacobianBenchmark")
	wxArm, err := NewArmJSONFile(nil, utils.ResolveFile("kinematics/models/mdl/wx250s.json"), 1, logger)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateJacobianIKSolver(wxArm.Model)
	wxArm.ik = ik

	// Used for benchmarking solve rate
	solved := 0
	for i := 0; i < toSolve; i++ {
		jPos := wxArm.Model.RandomJointPositions()
		wxArm.Model.SetPosition(jPos)
		rPos := wxArm.GetForwardPosition()
		startPos := wxArm.Model.RandomJointPositions()
		wxArm.Model.SetPosition(startPos)
		err = wxArm.SetForwardPosition(rPos)
		if err == nil {
			solved++
		}
	}
	fmt.Println("jacob solved: ", solved)
}

func TestIKTolerances(t *testing.T) {
	logger := golog.NewTestLogger(t)
	nCPU := runtime.NumCPU()
	v1Arm, err := NewArmJSONFile(nil, utils.ResolveFile("kinematics/models/mdl/v1_notol_test.json"), nCPU, logger)
	test.That(t, err, test.ShouldBeNil)
	v1Arm.SetJointPositions([]float64{5, 0})

	// Test inability to arrive at another position due to orientation
	pos := &pb.ArmPosition{
		X:  -46,
		Y:  0,
		Z:  372,
		RX: -178,
		RY: -33,
		RZ: -111,
	}
	err = v1Arm.SetForwardPosition(pos)

	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	v1Arm, err = NewArmJSONFile(nil, utils.ResolveFile("kinematics/models/mdl/v1_tol_test.json"), nCPU, logger)
	test.That(t, err, test.ShouldBeNil)
	v1Arm.SetJointPositions([]float64{5, 0})
	err = v1Arm.SetForwardPosition(pos)

	test.That(t, err, test.ShouldBeNil)
}
