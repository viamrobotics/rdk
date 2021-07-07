package kinematics

import (
	"math"
	"math/rand"
	"testing"

	"go.viam.com/core/utils"

	"go.viam.com/test"
)

func TestModelLoading(t *testing.T) {
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.OperationalDof(), test.ShouldEqual, 1)
	test.That(t, m.Dof(), test.ShouldEqual, 6)

	isValid := m.IsValid([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1})
	test.That(t, isValid, test.ShouldBeTrue)
	isValid = m.IsValid([]float64{0.1, 0.1, 0.1, 0.1, 0.1, 99.1})
	test.That(t, isValid, test.ShouldBeFalse)

	orig := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1}
	orig[5] += math.Pi * 2
	normalized := m.Normalize(orig)
	test.That(t, normalized[5], test.ShouldAlmostEqual, 0.1)

	randpos := m.RandomJointPositions(rand.New(rand.NewSource(1)))
	test.That(t, m.IsValid(randpos), test.ShouldBeTrue)
}
