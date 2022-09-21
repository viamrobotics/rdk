package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestConversions(t *testing.T) {
	// t.Run("test calculation", func(t *testing.T) {

	// dODt :=

	start := r3.Vector{X: 2, Y: 1, Z: 3}
	dt := 2.0

	for _, rate := range []struct {
		TestName    string
		AngularRate r3.Vector
	}{
		{"unitary roll", r3.Vector{X: 1, Y: 0, Z: 0}},
		{"unitary pitch", r3.Vector{X: 0, Y: 1, Z: 0}},
		{"unitary yaw", r3.Vector{X: 0, Y: 0, Z: 1}},
		{"roll", r3.Vector{X: 2, Y: 0, Z: 0}},
		{"pitch", r3.Vector{X: 0, Y: 4, Z: 0}},
		{"yaw", r3.Vector{X: 0, Y: 0, Z: 5}},
	} {
		t.Run(rate.TestName, func(t *testing.T) {
			// set up single axis frame speeds
			fin := start.Add(rate.AngularRate.Mul(dt))
			diff := fin.Sub(start)
			diffEu := &EulerAngles{Roll: diff.X, Pitch: diff.Y, Yaw: diff.Z}

			qav := QuatToAngVel(diffEu.Quaternion(), dt)
			oav := OrientationToAngularVel(diffEu, dt)
			eav := EulerToAngVel(*diffEu, dt)
			rav := RotMatToAngVel(*diffEu.RotationMatrix(), dt)

			t.Run("quaternion", func(t *testing.T) {
				test.That(t, qav, test.ShouldResemble, R3ToAngVel(rate.AngularRate))
			})
			t.Run("orientation", func(t *testing.T) {
				test.That(t, oav, test.ShouldResemble, R3ToAngVel(rate.AngularRate))
			})
			t.Run("euler", func(t *testing.T) {
				test.That(t, eav, test.ShouldResemble, R3ToAngVel(rate.AngularRate))
			})
			t.Run("rotation matrix", func(t *testing.T) {
				test.That(t, rav, test.ShouldResemble, R3ToAngVel(rate.AngularRate))
			})
		})

	}
}
