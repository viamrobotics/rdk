package arm

import (
	"fmt"
	"testing"

	"github.com/edaniels/test"
	"github.com/viamrobotics/robotcore/testutils"
)

// This should test all of the kinematics functions
func TestJacIKinematics(t *testing.T) {
	//~ wxArm, err := arm.NewRobot(testutils.ResolveFile("kinematics/models/mdl/wx250s_test.json"))
	//~ test.That(t, err, test.ShouldBeNil)
	evaArm, err := NewRobot(testutils.ResolveFile("kinematics/models/mdl/eva.json"))
	test.That(t, err, test.ShouldBeNil)

	//~ zeroPos := evaArm.GetJointPositions()

	evaArm.SetJointPositions([]float64{69.35309996071989, 28.752097952708045, -101.57720046840646, 0.9393597585332618, -73.96221972947882, 0.03845332136188379})

	fmt.Println(evaArm.GetForwardPosition())
	// Test ability to arrive at a small X shift ahead
	pos := Position{-46.445827416798814, -133.99229347583582, 372.4849299627513, -178.88747811107424, -33.160094626838045, -111.02282693533935}
	err = evaArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)

	// Test a larger X axis movement
	pos = Position{-66.445827416798814, -133.99229347583582, 372.4849299627513, -178.88747811107424, -33.160094626838045, -111.02282693533935}
	err = evaArm.SetForwardPosition(pos)
	test.That(t, err, test.ShouldBeNil)

	// Test we are able to solve random valid positions from other random valid positions
	for i := 0; i < 1000; i++ {
		jPos := evaArm.Model.RandomJointPositions()
		evaArm.SetJointPositions(jPos)
		rPos := evaArm.GetForwardPosition()
		evaArm.SetJointPositions(evaArm.Model.RandomJointPositions())
		err = evaArm.SetForwardPosition(rPos)
		test.That(t, err, test.ShouldBeNil)
	}
}
