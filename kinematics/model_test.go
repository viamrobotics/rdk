package kinematics

import (
	"math"
	"math/rand"
	"testing"

	"go.viam.com/core/referenceframe"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"go.viam.com/test"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

func TestModelLoading(t *testing.T) {
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.OperationalDof(), test.ShouldEqual, 1)
	test.That(t, len(m.Dof()), test.ShouldEqual, 6)

	isValid := m.AreJointPositionsValid([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1})
	test.That(t, isValid, test.ShouldBeTrue)
	isValid = m.AreJointPositionsValid([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 99.1})
	test.That(t, isValid, test.ShouldBeFalse)

	orig := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1}
	orig[5] += math.Pi * 2
	orig[4] -= math.Pi * 4

	randpos := m.GenerateRandomJointPositions(rand.New(rand.NewSource(1)))
	test.That(t, m.AreJointPositionsValid(randpos), test.ShouldBeTrue)

	m.SetName("foo")
	test.That(t, m.Name(), test.ShouldEqual, "foo")
}

func TestJoint(t *testing.T) {
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	joints := m.Joints()
	test.That(t, len(joints), test.ShouldEqual, 6)
	pose, err := joints[0].Transform([]referenceframe.Input{{0}})
	test.That(t, err, test.ShouldBeNil)
	firstJquat := spatialmath.NewDualQuaternionFromPose(pose).Number
	firstJquatExpect := dualquat.Number{quat.Number{1, 0, 0, 0}, quat.Number{0, 0, 0, 0}}
	test.That(t, firstJquat, test.ShouldResemble, firstJquatExpect)

	pose, err = joints[0].Transform([]referenceframe.Input{{1.5708}})
	test.That(t, err, test.ShouldBeNil)
	firstJangle := spatialmath.NewDualQuaternionFromPose(pose).Number
	firstJangleExpect := dualquat.Number{quat.Number{0.7071054825112365, 0, 0, 0.7071080798594737}, quat.Number{0, 0, 0, 0}}
	test.That(t, firstJangle, test.ShouldResemble, firstJangleExpect)
}
