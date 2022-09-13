package varm

import (
	"testing"

	"go.viam.com/test"
)

var epsilon = .01

func TestArmMath1(t *testing.T) {
	test.That(t, computeInnerJointAngle(0, 0), test.ShouldAlmostEqual, 0.0, epsilon)
	test.That(t, computeInnerJointAngle(0, -90), test.ShouldAlmostEqual, -90.0, epsilon)
	test.That(t, computeInnerJointAngle(-45, -90), test.ShouldAlmostEqual, -135.0, epsilon)
	test.That(t, computeInnerJointAngle(45, -90), test.ShouldAlmostEqual, -45.0, epsilon)

	j := joint{
		posMin: 0.0,
		posMax: 1.0,
		degMin: 0.0,
		degMax: 90.0,
	}

	test.That(t, j.positionToValues(0.0), test.ShouldAlmostEqual, 0.0, epsilon)
	test.That(t, j.positionToValues(0.5), test.ShouldAlmostEqual, 45, epsilon)
	test.That(t, j.positionToValues(1.0), test.ShouldAlmostEqual, 90, epsilon)

	test.That(t, j.degreesToPosition(0.0), test.ShouldAlmostEqual, 0.0, epsilon)
	test.That(t, j.degreesToPosition(45.0), test.ShouldAlmostEqual, 0.5, epsilon)
	test.That(t, j.degreesToPosition(90.0), test.ShouldAlmostEqual, 1.0, epsilon)
}
