package api

import (
	"math"
	"testing"

	"go.viam.com/robotcore/utils"

	"github.com/edaniels/test"
)

func TestArmPosition(t *testing.T) {
	p := NewPositionFromMetersAndRadians(1.0, 2.0, 3.0, 0, math.Pi/2, math.Pi)

	test.That(t, p.RX, test.ShouldEqual, 0.0)
	test.That(t, p.RY, test.ShouldEqual, 90.0)
	test.That(t, p.RZ, test.ShouldEqual, 180.0)

	test.That(t, utils.DegToRad(p.RX), test.ShouldEqual, 0.0)
	test.That(t, utils.DegToRad(p.RY), test.ShouldEqual, math.Pi/2)
	test.That(t, utils.DegToRad(p.RZ), test.ShouldEqual, math.Pi)
}

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	test.That(t, j.Degrees[0], test.ShouldEqual, 0.0)
	test.That(t, j.Degrees[1], test.ShouldEqual, 180.0)
	test.That(t, JointPositionsToRadians(j), test.ShouldResemble, in)
}
