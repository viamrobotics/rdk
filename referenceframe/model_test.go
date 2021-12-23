package referenceframe

import (
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"

	"go.viam.com/test"
)

func TestModelLoading(t *testing.T) {
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Name(), test.ShouldEqual, "wx250s")

	test.That(t, m.OperationalDoF(), test.ShouldEqual, 1)
	test.That(t, len(m.DoF()), test.ShouldEqual, 6)

	isValid := m.AreJointPositionsValid([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1})
	test.That(t, isValid, test.ShouldBeTrue)
	isValid = m.AreJointPositionsValid([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 99.1})
	test.That(t, isValid, test.ShouldBeFalse)

	orig := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1}
	orig[5] += math.Pi * 2
	orig[4] -= math.Pi * 4

	randpos := m.GenerateRandomJointPositions(rand.New(rand.NewSource(1)))
	test.That(t, m.AreJointPositionsValid(randpos), test.ShouldBeTrue)

	m, err = ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), "foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Name(), test.ShouldEqual, "foo")
}

func TestTransform(t *testing.T) {
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	joints := []Frame{}
	for _, tform := range m.OrdTransforms {
		if len(tform.DoF()) > 0 {
			joints = append(joints, tform)
		}
	}
	test.That(t, len(joints), test.ShouldEqual, 6)
	pose, err := joints[0].Transform([]Input{{0}})
	test.That(t, err, test.ShouldBeNil)
	firstJov := pose.Orientation().OrientationVectorRadians()
	firstJovExpect := &spatialmath.OrientationVector{Theta: 0, OX: 0, OY: 0, OZ: 1}
	test.That(t, firstJov, test.ShouldResemble, firstJovExpect)

	pose, err = joints[0].Transform([]Input{{1.5708}})
	test.That(t, err, test.ShouldBeNil)
	firstJov = pose.Orientation().OrientationVectorRadians()
	firstJovExpect = &spatialmath.OrientationVector{Theta: 1.5708, OX: 0, OY: 0, OZ: 1}
	test.That(t, firstJov.Theta, test.ShouldAlmostEqual, firstJovExpect.Theta)
	test.That(t, firstJov.OX, test.ShouldAlmostEqual, firstJovExpect.OX)
	test.That(t, firstJov.OY, test.ShouldAlmostEqual, firstJovExpect.OY)
	test.That(t, firstJov.OZ, test.ShouldAlmostEqual, firstJovExpect.OZ)
}

func TestVerboseTransform(t *testing.T) {
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

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
