package kinematics

import (
	//~ "fmt"
	"testing"

	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

const ToSolve = 100

// This should test all of the kinematics functions
func TestCombinedIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	wxArm, err := NewArm(nil, utils.ResolveFile("kinematics/models/mdl/wx250s.json"), 4, logger)
	test.That(t, err, test.ShouldBeNil)
	wxArm.SetJointPositions([]float64{69.35309996071989, 28.752097952708045, -101.57720046840646, 0.9393597585332618, -73.96221972947882, 0.03845332136188379})

	// Test ability to arrive at a small X shift ahead
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

	// Test a larger movement- currently does not find with 4 cores but will with 8
	//~ pos = &pb.ArmPosition{
	//~	X:  -66.445827416798814,
	//~	Y:  -133.99229347583582,
	//~	Z:  372.4849299627513,
	//~	RX: -178.88747811107424,
	//~	RY: -33.160094626838045,
	//~	RZ: -111.02282693533935,
	//~}
	//~ err = wxArm.SetForwardPosition(pos)
	//~ test.That(t, err, test.ShouldBeNil)

	// Test we are able to solve random valid positions from other random valid positions
	// Used for benchmarking solve rate
	//~ solved := 0
	//~ for i := 0; i < ToSolve; i++ {
	//~ fmt.Println(i)
	//~ jPos := wxArm.Model.RandomJointPositions()
	//~ wxArm.Model.SetPosition(jPos)
	//~ rPos := wxArm.GetForwardPosition()
	//~ startPos := wxArm.Model.RandomJointPositions()
	//~ wxArm.Model.SetPosition(startPos)
	//~ err = wxArm.SetForwardPosition(rPos)
	//~ if err == nil {
	//~ solved++
	//~ } else {
	//~ fmt.Println("from: ", startPos)
	//~ fmt.Println("to: ", jPos)
	//~ fmt.Println(err)
	//~ }
	//~ }
	//~ fmt.Println("combined solved: ", solved)
}

func TestNloptIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	wxArm, err := NewArm(nil, utils.ResolveFile("kinematics/models/mdl/wx250s.json"), 1, logger)
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

	// Used for benchmarking solve rate
	//~ solved := 0
	//~ for i := 0; i < ToSolve; i++ {
	//~ jPos := wxArm.Model.RandomJointPositions()
	//~ wxArm.Model.SetPosition(jPos)
	//~ rPos := wxArm.GetForwardPosition()
	//~ startPos := wxArm.Model.RandomJointPositions()
	//~ wxArm.Model.SetPosition(startPos)
	//~ err = wxArm.SetForwardPosition(rPos)
	//~ if err == nil {
	//~ solved++
	//~ }
	//~ }
	//~ fmt.Println("nlopt solved: ", solved)
}

func TestJacobianIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	wxArm, err := NewArm(nil, utils.ResolveFile("kinematics/models/mdl/wx250s.json"), 1, logger)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateJacobianIKSolver(wxArm.Model)
	wxArm.ik = ik

	pos := &pb.ArmPosition{X: 80, Y: -370, Z: 355, RX: 15, RY: 0, RZ: 0}
	err = wxArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)

	// Used for benchmarking solve rate
	//~ solved := 0
	//~ for i := 0; i < ToSolve; i++ {
	//~ jPos := wxArm.Model.RandomJointPositions()
	//~ wxArm.Model.SetPosition(jPos)
	//~ rPos := wxArm.GetForwardPosition()
	//~ startPos := wxArm.Model.RandomJointPositions()
	//~ wxArm.Model.SetPosition(startPos)
	//~ err = wxArm.SetForwardPosition(rPos)
	//~ if err == nil {
	//~ solved++
	//~ }
	//~ }
	//~ fmt.Println("jacob solved: ", solved)
}
