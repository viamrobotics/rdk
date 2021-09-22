package config

import (
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/spatialmath"
)

func TestOrientation(t *testing.T) {
	fc := &FrameConfig{
		Parent:      "a",
		Translation: Translation{1, 2, 3},
	}

	// empty orientation
	o := fc.Orientation()
	test.That(t, o.EulerAngles(), test.ShouldResemble, &spatialmath.EulerAngles{Roll: 0, Pitch: 0, Yaw: 0})

	// OrientationVec 90 degrees around z axis
	ov := &spatialmath.OrientationVec{Theta: math.Pi / 2., OX: 0, OY: 0, OZ: 1}
	fc.OVDegrees = nil
	fc.OVRadians = ov
	o = fc.Orientation()
	test.That(t, o.OV().Theta, test.ShouldAlmostEqual, math.Pi/2.)
	test.That(t, o.OV().OX, test.ShouldAlmostEqual, 0)
	test.That(t, o.OV().OY, test.ShouldAlmostEqual, 0)
	test.That(t, o.OV().OZ, test.ShouldAlmostEqual, 1)
	test.That(t, o.EulerAngles().Roll, test.ShouldAlmostEqual, 0)
	test.That(t, o.EulerAngles().Pitch, test.ShouldAlmostEqual, 0)
	test.That(t, o.EulerAngles().Yaw, test.ShouldAlmostEqual, math.Pi/2.)

	// OrientationVecDegrees 90 degrees around z axis
	ovd := &spatialmath.OrientationVecDegrees{Theta: 89.99999999999999, OX: 0, OY: 0, OZ: 1.0000000000000002}
	fc.OVDegrees = ovd
	o = fc.Orientation()
	test.That(t, o.OVD(), test.ShouldResemble, ovd)
	test.That(t, o.EulerAngles().Roll, test.ShouldAlmostEqual, 0)
	test.That(t, o.EulerAngles().Pitch, test.ShouldAlmostEqual, 0)
	test.That(t, o.EulerAngles().Yaw, test.ShouldAlmostEqual, math.Pi/2.)

	// Axis angle 90 degrees around z axis
	aa := &spatialmath.R4AA{Theta: math.Pi / 2., RX: 0, RY: 0, RZ: 1}
	fc.OVRadians = nil
	fc.AxisAngles = aa
	o = fc.Orientation()
	test.That(t, o.AxisAngles().Theta, test.ShouldAlmostEqual, math.Pi/2.)
	test.That(t, o.AxisAngles().RX, test.ShouldAlmostEqual, 0)
	test.That(t, o.AxisAngles().RY, test.ShouldAlmostEqual, 0)
	test.That(t, o.AxisAngles().RZ, test.ShouldAlmostEqual, 1)
	test.That(t, o.EulerAngles().Roll, test.ShouldAlmostEqual, 0)
	test.That(t, o.EulerAngles().Pitch, test.ShouldAlmostEqual, 0)
	test.That(t, o.EulerAngles().Yaw, test.ShouldAlmostEqual, math.Pi/2.)

	// Euler angle 90 degrees around z axis
	ea := &spatialmath.EulerAngles{Roll: 0, Pitch: 0, Yaw: math.Pi / 2.}
	fc.AxisAngles = nil
	fc.EulerAngles = ea
	o = fc.Orientation()
	test.That(t, o.EulerAngles().Roll, test.ShouldAlmostEqual, 0)
	test.That(t, o.EulerAngles().Pitch, test.ShouldAlmostEqual, 0)
	test.That(t, o.EulerAngles().Yaw, test.ShouldAlmostEqual, math.Pi/2.)

}
