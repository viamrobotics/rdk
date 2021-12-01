package collision

import (
	"testing"

	"github.com/golang/geo/r3"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"
	"go.viam.com/test"
)

func TestSelfCollision(t *testing.T) {
	modelUR5e, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelUR5e, urOffset)

	poses, err := m.VerboseTransform([]Input{{0.0}, {0.0}, {0.0}, {0.0}, {0.0}, {0.0}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(poses), test.ShouldEqual, len(m.OrdTransforms))

	shoulderExpect := spatialmath.NewPoseFromPoint(r3.Vector{0.0, 0.0, 110.25})
	test.That(t, spatialmath.AlmostCoincident(poses["wx250s:shoulder"], shoulderExpect), test.ShouldBeTrue)
	upperArmExpect := spatialmath.NewPoseFromPoint(r3.Vector{50.0, 0.0, 360.25})
	test.That(t, spatialmath.AlmostCoincident(poses["wx250s:upper_arm"], upperArmExpect), test.ShouldBeTrue)
	forearmPoseExpect := spatialmath.NewPoseFromPoint(r3.Vector{300.0, 0.0, 360.25})
	test.That(t, spatialmath.AlmostCoincident(poses["wx250s:forearm"], forearmPoseExpect), test.ShouldBeTrue)
}
