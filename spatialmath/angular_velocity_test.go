package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestConversions(t *testing.T) {
	dt := 2.0

	for _, angVs := range []struct {
		testName string
		rate     r3.Vector
	}{
		{"unitary roll", r3.Vector{X: 1, Y: 0, Z: 0}},
		{"unitary pitch", r3.Vector{X: 0, Y: 1, Z: 0}},
		{"unitary yaw", r3.Vector{X: 0, Y: 0, Z: 1}},
		{"roll", r3.Vector{X: 2, Y: 0, Z: 0}},
		{"pitch", r3.Vector{X: 0, Y: 4, Z: 0}},
		{"yaw", r3.Vector{X: 0, Y: 0, Z: 5}},
	} {
		t.Run(angVs.testName, func(t *testing.T) {
			// set up single axis frame speeds
			diffEu := &EulerAngles{
				Roll:  dt * angVs.rate.X,
				Pitch: dt * angVs.rate.Y,
				Yaw:   dt * angVs.rate.Z,
			}
			oav := OrientationToAngularVel(diffEu, dt)
			eav := EulerToAngVel(*diffEu, dt)

			t.Run("orientation", func(t *testing.T) {
				test.That(t, oav, test.ShouldResemble, *R3ToAngVel(angVs.rate))
			})
			t.Run("euler", func(t *testing.T) {
				test.That(t, eav, test.ShouldResemble, *R3ToAngVel(angVs.rate))
			})
		})
	}
}
