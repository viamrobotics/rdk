package boat

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

func TestComputeNextPower(t *testing.T) {
	_, a := computeNextPower(
		&boatState{
			velocityAngularGoal: r3.Vector{Z: 45},
		},
		spatialmath.AngularVelocity{},
	)
	test.That(t, a.Z, test.ShouldEqual, .125)

	_, a = computeNextPower(
		&boatState{
			velocityAngularGoal: r3.Vector{Z: -45},
		},
		spatialmath.AngularVelocity{},
	)
	test.That(t, a.Z, test.ShouldEqual, -.125)

	_, a2 := computeNextPower(
		&boatState{
			lastPowerAngular:    r3.Vector{Z: .3},
			velocityAngularGoal: r3.Vector{Z: 45},
		},
		spatialmath.AngularVelocity{Z: 30},
	)
	test.That(t, a2.Z, test.ShouldBeGreaterThan, a.Z)
	test.That(t, a2.Z, test.ShouldBeGreaterThan, .3)

	_, a = computeNextPower(
		&boatState{
			lastPowerAngular:    r3.Vector{Z: -.2},
			velocityAngularGoal: r3.Vector{Z: 45},
		},
		spatialmath.AngularVelocity{Z: -30},
	)
	test.That(t, a.Z, test.ShouldBeGreaterThan, 0)
}
