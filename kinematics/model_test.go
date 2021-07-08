package kinematics

import (
	"math"
	"math/rand"
	"testing"

	"go.viam.com/core/utils"

	"go.viam.com/test"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

func TestModelLoading(t *testing.T) {
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.OperationalDof(), test.ShouldEqual, 1)
	test.That(t, m.Dof(), test.ShouldEqual, 6)

	isValid := m.AreJointPositionsValid([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1})
	test.That(t, isValid, test.ShouldBeTrue)
	isValid = m.AreJointPositionsValid([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 99.1})
	test.That(t, isValid, test.ShouldBeFalse)

	orig := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1}
	orig[5] += math.Pi * 2
	orig[4] -= math.Pi * 4
	normalized := m.Normalize(orig)
	test.That(t, normalized[4], test.ShouldAlmostEqual, 0.1)
	test.That(t, normalized[5], test.ShouldAlmostEqual, 0.1)

	randpos := m.GenerateRandomJointPositions(rand.New(rand.NewSource(1)))
	test.That(t, m.AreJointPositionsValid(randpos), test.ShouldBeTrue)
}

func TestJoint(t *testing.T) {
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	joints := m.Joints()
	test.That(t, len(joints), test.ShouldEqual, 6)
	firstJquat := joints[0].Quaternion().Quat
	firstJquatExpect := dualquat.Number{quat.Number{0, 0, 0, 1}, quat.Number{0, 0, 0, 0}}
	test.That(t, firstJquat, test.ShouldResemble, firstJquatExpect)

	firstJangle := joints[0].AngleQuaternion([]float64{1.5708}).Quat
	firstJangleExpect := dualquat.Number{quat.Number{0.7071054825112365, 0, 0, 0.7071080798594737}, quat.Number{0, 0, 0, 0}}
	test.That(t, firstJangle, test.ShouldResemble, firstJangleExpect)
}
