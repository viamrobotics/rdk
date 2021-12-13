package collision

import (
	"testing"

	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/utils"
	"go.viam.com/test"
)

func TestSelfCollision(t *testing.T) {
	modelUR5e, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	inputs := []frame.Input{{0.0}, {0.0}, {0.0}, {0.0}, {0.0}, {0.0}}

	collisions, err := SelfCollision(modelUR5e, inputs)
	test.That(t, collisions, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)

	// shoulderExpect := spatialmath.NewPoseFromPoint(r3.Vector{0.0, 0.0, 110.25})
	// test.That(t, spatialmath.AlmostCoincident(poses["wx250s:shoulder"], shoulderExpect), test.ShouldBeTrue)
	// upperArmExpect := spatialmath.NewPoseFromPoint(r3.Vector{50.0, 0.0, 360.25})
	// test.That(t, spatialmath.AlmostCoincident(poses["wx250s:upper_arm"], upperArmExpect), test.ShouldBeTrue)
	// forearmPoseExpect := spatialmath.NewPoseFromPoint(r3.Vector{300.0, 0.0, 360.25})
	// test.That(t, spatialmath.AlmostCoincident(poses["wx250s:forearm"], forearmPoseExpect), test.ShouldBeTrue)
}
