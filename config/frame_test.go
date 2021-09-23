package config

import (
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/spatialmath"
)

func TestOrientationEmpty(t *testing.T) {
	fc := &FrameConfig{
		Parent:      "a",
		Translation: Translation{1, 2, 3},
	}

	// empty orientation
	o := fc.Orientation()
	test.That(t, o.EulerAngles(), test.ShouldResemble, &spatialmath.EulerAngles{Roll: 0, Pitch: 0, Yaw: 0})
}

func TestOrientationOV(t *testing.T) {
	fc := &FrameConfig{
		Parent:      "a",
		Translation: Translation{1, 2, 3},
	}

	ov := &spatialmath.OrientationVec{Theta: math.Pi / 2., OX: 0, OY: 0, OZ: 1}
	fc.OVRadians = ov
	o := fc.Orientation()
	test.That(t, o.OV().Theta, test.ShouldAlmostEqual, ov.Theta)
	test.That(t, o.OV().OX, test.ShouldAlmostEqual, ov.OX)
	test.That(t, o.OV().OY, test.ShouldAlmostEqual, ov.OY)
	test.That(t, o.OV().OZ, test.ShouldAlmostEqual, ov.OZ)
}

func TestOrientationOVD(t *testing.T) {
	fc := &FrameConfig{
		Parent:      "a",
		Translation: Translation{1, 2, 3},
	}

	ovd := &spatialmath.OrientationVecDegrees{Theta: 60, OX: 0, OY: 0, OZ: 1}
	fc.OVDegrees = ovd
	o := fc.Orientation()
	test.That(t, o.OVD().Theta, test.ShouldAlmostEqual, ovd.Theta)
	test.That(t, o.OVD().OX, test.ShouldAlmostEqual, ovd.OX)
	test.That(t, o.OVD().OY, test.ShouldAlmostEqual, ovd.OY)
	test.That(t, o.OVD().OZ, test.ShouldAlmostEqual, ovd.OZ)
}

func TestOrientationAxisAngle(t *testing.T) {
	fc := &FrameConfig{
		Parent:      "a",
		Translation: Translation{1, 2, 3},
	}
	aa := &spatialmath.R4AA{Theta: math.Pi / 4., RX: 0, RY: 0, RZ: 1}
	fc.AxisAngles = aa
	o := fc.Orientation()
	test.That(t, o.AxisAngles().Theta, test.ShouldAlmostEqual, aa.Theta)
	test.That(t, o.AxisAngles().RX, test.ShouldAlmostEqual, aa.RX)
	test.That(t, o.AxisAngles().RY, test.ShouldAlmostEqual, aa.RY)
	test.That(t, o.AxisAngles().RZ, test.ShouldAlmostEqual, aa.RZ)
}

func TestOrientationEuler(t *testing.T) {
	fc := &FrameConfig{
		Parent:      "a",
		Translation: Translation{1, 2, 3},
	}
	ea := &spatialmath.EulerAngles{Roll: 0, Pitch: 0, Yaw: math.Pi / 6.}
	fc.EulerAngles = ea
	o := fc.Orientation()
	test.That(t, o.EulerAngles().Roll, test.ShouldAlmostEqual, ea.Roll)
	test.That(t, o.EulerAngles().Pitch, test.ShouldAlmostEqual, ea.Pitch)
	test.That(t, o.EulerAngles().Yaw, test.ShouldAlmostEqual, ea.Yaw)

}
