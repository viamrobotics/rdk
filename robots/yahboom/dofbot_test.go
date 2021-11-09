package yahboom

import (
	"testing"

	"go.viam.com/test"
)

func TestJointConfig(t *testing.T) {
	test.That(t, joints[0].toDegrees(joints[0].toHw(0)), test.ShouldAlmostEqual, 0)
	test.That(t, joints[0].toDegrees(joints[0].toHw(45)), test.ShouldAlmostEqual, 45)
	test.That(t, joints[0].toDegrees(joints[0].toHw(90)), test.ShouldAlmostEqual, 90)
	test.That(t, joints[0].toDegrees(joints[0].toHw(135)), test.ShouldAlmostEqual, 135)
	test.That(t, joints[0].toDegrees(joints[0].toHw(200)), test.ShouldAlmostEqual, 200, .1)
	test.That(t, joints[0].toDegrees(joints[0].toHw(300)), test.ShouldAlmostEqual, 300, .1)
	test.That(t, joints[0].toDegrees(joints[0].toHw(350)), test.ShouldAlmostEqual, 350, .1)
}
