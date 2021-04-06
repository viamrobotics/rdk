package kinematics

import (
	//~ "fmt"
	"testing"

	"github.com/edaniels/test"
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/testutils"
)

const ToSolve = 100

// This should test all of the kinematics functions
func TestCombinedIKinematics(t *testing.T) {
	wxArm, err := NewArm(nil, testutils.ResolveFile("kinematics/models/mdl/wx250s.json"), 4)
	test.That(t, err, test.ShouldBeNil)
	wxArm.SetJointPositions([]float64{69.35309996071989, 28.752097952708045, -101.57720046840646, 0.9393597585332618, -73.96221972947882, 0.03845332136188379})

	// Test ability to arrive at a small X shift ahead
	pos := api.ArmPosition{-46.445827416798814, -133.99229347583582, 372.4849299627513, -178.88747811107424, -33.160094626838045, -111.02282693533935}
	err = wxArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)

	// Test a larger movement- currently does not find with 4 cores but will with 8
	//~ pos = api.ArmPosition{-66.445827416798814, -133.99229347583582, 372.4849299627513, -178.88747811107424, -33.160094626838045, -111.02282693533935}
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
	wxArm, err := NewArm(nil, testutils.ResolveFile("kinematics/models/mdl/wx250s.json"), 1)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateNloptIKSolver(wxArm.Model)
	wxArm.ik = ik

	pos := api.ArmPosition{80, -370, 355, 15, 0, 0}
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
	wxArm, err := NewArm(nil, testutils.ResolveFile("kinematics/models/mdl/wx250s.json"), 1)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateJacobianIKSolver(wxArm.Model)
	wxArm.ik = ik

	pos := api.ArmPosition{80, -370, 355, 15, 0, 0}
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
