package kinematics

import (
	"fmt"
	"testing"

	"go.viam.com/robotcore/kinematics/kinmath"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

// This should test all of the kinematics functions
func TestCombinedIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	evaArm, err := NewArm(nil, utils.ResolveFile("kinematics/models/mdl/wx250s_test.json"), 1, logger)
	test.That(t, err, test.ShouldBeNil)
	//~ 	evaArm, err := NewRobot(utils.ResolveFile("kinematics/models/mdl/eva.json"), 1)
	//~ 	test.That(t, err, test.ShouldBeNil)

	evaArm.SetJointPositions([]float64{69.35309996071989, 28.752097952708045, -101.57720046840646, 0.9393597585332618, -73.96221972947882, 0.03845332136188379})

	// Test ability to arrive at a small X shift ahead
	pos := &pb.ArmPosition{
		X:  -46.445827416798814,
		Y:  -133.99229347583582,
		Z:  372.4849299627513,
		RX: -178.88747811107424,
		RY: -33.160094626838045,
		RZ: -111.02282693533935,
	}
	err = evaArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)

	// Test a larger X axis movement
	pos = &pb.ArmPosition{
		X:  -66.445827416798814,
		Y:  -133.99229347583582,
		Z:  372.4849299627513,
		RX: -178.88747811107424,
		RY: -33.160094626838045,
		RZ: -111.02282693533935,
	}
	err = evaArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)

	// Test we are able to solve random valid positions from other random valid positions
	solved := 0
	for i := 0; i < 100; i++ {
		fmt.Println(i)
		jPos := evaArm.Model.RandomJointPositions()
		evaArm.Model.SetPosition(jPos)
		rPos := evaArm.GetForwardPosition()
		startPos := evaArm.Model.RandomJointPositions()
		evaArm.Model.SetPosition(startPos)
		err = evaArm.SetForwardPosition(rPos)
		if err == nil {
			solved++
		} else {
			fmt.Println("from: ", startPos)
			fmt.Println("to: ", jPos)
			fmt.Println(err)
		}
	}
	fmt.Println("solved: ", solved)
}

func TestNloptIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	wxArm, err := NewArm(nil, utils.ResolveFile("kinematics/models/mdl/wx250s_test.json"), 1, logger)
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
	transform := kinmath.NewTransformFromRotation(pos.RX, pos.RY, pos.RZ)
	transform.SetX(pos.X)
	transform.SetY(pos.Y)
	transform.SetZ(pos.Z)

	ik.AddGoal(transform, 0)
	solved := ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)
}

//~ func TestJacobianIKinematics(t *testing.T) {
//~ 	wxArm, err := NewRobot(utils.ResolveFile("kinematics/models/mdl/wx250s_test.json"), 1)
//~ 	test.That(t, err, test.ShouldBeNil)
//~ 	ik := kinematics.CreateJacobianIKSolver(wxArm.Model)

//~ 	pos := Position{1, -370, 355, 0, 0, 0}
//~ 	transform := kinmath.NewTransformFromRotation(pos.Rx, pos.Ry, pos.Rz)
//~ 	transform.SetX(pos.X)
//~ 	transform.SetY(pos.Y)
//~ 	transform.SetZ(pos.Z)

//~ 	ik.AddGoal(transform, 0)
//~ 	solved := ik.Solve()
//~ 	test.That(t, solved, test.ShouldBeTrue)
//~ }
