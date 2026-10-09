package spatialmath

import (
	"fmt"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

// Repro for https://github.com/viamrobotics/rdk/issues/6627. An orientation vector whose pointing
// direction is within ~0.81 degrees of +/-Z decodes as if it leaned toward +X: Quaternion() drops
// the longitude while 1-|OZ| <= defaultAngleEpsilon, but defaultAngleEpsilon is an angle (1e-4 rad)
// and 1-|OZ| is roughly half the tilt squared, so the "pole" reaches acos(1-1e-4) = 0.81 degrees.

// TestOrientationVectorPoleRadius, with its example leaning the other way. The original passes
// only because its tilt leans toward +X/+Y, close to the +X the decoder assumes.
func TestOrientationVectorPoleRadiusMirrored(t *testing.T) {
	ov := &OrientationVectorDegrees{Theta: 90.2029644505, OX: -0.0050164674, OY: -0.0079070413, OZ: 0.9999561559}
	q := Quaternion(ov.Quaternion())
	composedOv := q.OrientationVectorDegrees()
	test.That(t, composedOv.Theta, test.ShouldAlmostEqual, ov.Theta, 0.01)
	test.That(t, composedOv.OX, test.ShouldAlmostEqual, ov.OX, 0.01)
	test.That(t, composedOv.OY, test.ShouldAlmostEqual, ov.OY, 0.01)
	test.That(t, composedOv.OZ, test.ShouldAlmostEqual, ov.OZ, 0.01)
}

// nearPole is an orientation vector pointing tiltDeg away from +Z (or -Z when down), leaning
// toward azimuthDeg in the XY plane.
func nearPole(down bool, tiltDeg, azimuthDeg, thetaDeg float64) *OrientationVectorDegrees {
	tilt, az := utils.DegToRad(tiltDeg), utils.DegToRad(azimuthDeg)
	z := math.Cos(tilt)
	if down {
		z = -z
	}
	return &OrientationVectorDegrees{OX: math.Sin(tilt) * math.Cos(az), OY: math.Sin(tilt) * math.Sin(az), OZ: z, Theta: thetaDeg}
}

func pointingDirection(o Orientation) r3.Vector {
	return QuatToRotationMatrix(o.Quaternion()).Col(2)
}

// The documented meaning of ov_degrees (docs.viam.com/motion-planning/reference/orientation-vectors):
// "x, y, z: the direction the component's +z axis points ... th: rotation in degrees around that
// same pointing direction". Whatever th is, the decoded +z axis must point along (x, y, z).
// Checked on the three ways an orientation vector is decoded: directly, from an API Pose, and
// from a frame's orientation in machine config.
func TestOrientationVectorPointsAlongXYZNearPoles(t *testing.T) {
	decoders := map[string]func(ov *OrientationVectorDegrees) Orientation{
		"OrientationVectorDegrees": func(ov *OrientationVectorDegrees) Orientation { return ov },
		"NewPoseFromProtobuf": func(ov *OrientationVectorDegrees) Orientation {
			return NewPoseFromProtobuf(&commonpb.Pose{OX: ov.OX, OY: ov.OY, OZ: ov.OZ, Theta: ov.Theta}).Orientation()
		},
		"OrientationConfig": func(ov *OrientationVectorDegrees) Orientation {
			cfg := OrientationConfig{
				Type:  OrientationVectorDegreesType,
				Value: map[string]any{"x": ov.OX, "y": ov.OY, "z": ov.OZ, "th": ov.Theta},
			}
			o, err := cfg.ParseConfig()
			test.That(t, err, test.ShouldBeNil)
			return o
		},
	}
	for name, decode := range decoders {
		t.Run(name, func(t *testing.T) {
			worst, worstCase := 0.0, ""
			for _, down := range []bool{false, true} {
				for _, tilt := range []float64{0.1, 0.5, 0.8, 2} {
					for az := 0.; az < 360; az += 45 {
						for _, theta := range []float64{0, 90} {
							ov := nearPole(down, tilt, az, theta)
							want := r3.Vector{X: ov.OX, Y: ov.OY, Z: ov.OZ}.Normalize()
							got := pointingDirection(decode(ov))
							if errDeg := utils.RadToDeg(float64(got.Angle(want))); errDeg > worst {
								worst = errDeg
								worstCase = fmt.Sprintf("tilt %.1f deg from %s toward azimuth %.0f deg, th %.0f: +z points %.3f deg off",
									tilt, map[bool]string{false: "+Z", true: "-Z"}[down], az, theta, errDeg)
							}
						}
					}
				}
			}
			test.That(t, worstCase, test.ShouldBeEmpty)
		})
	}
}

// Every pose sent over the API goes through PoseToProtobuf and NewPoseFromProtobuf. Orientations
// a fraction of a degree from vertical (a tool pointing "straight down") must survive the trip.
func TestPoseProtobufRoundTripNearPoles(t *testing.T) {
	worst, worstCase := 0.0, ""
	for _, down := range []bool{false, true} {
		for _, tilt := range []float64{0.05, 0.2, 0.5, 0.8, 2} {
			for az := 0.; az < 360; az += 45 {
				// Rz(az) Ry(lean) Rz(30 deg): +z tilted from +Z (or -Z) toward az, then spun.
				lean := utils.DegToRad(tilt)
				if down {
					lean = math.Pi - lean
				}
				o := Compose(
					NewPoseFromOrientation(&R4AA{Theta: utils.DegToRad(az), RZ: 1}),
					Compose(
						NewPoseFromOrientation(&R4AA{Theta: lean, RY: 1}),
						NewPoseFromOrientation(&R4AA{Theta: utils.DegToRad(30), RZ: 1}),
					),
				)
				back := NewPoseFromProtobuf(PoseToProtobuf(o))
				errDeg := utils.RadToDeg(math.Abs(OrientationBetween(o.Orientation(), back.Orientation()).AxisAngles().Theta))
				if !OrientationAlmostEqual(o.Orientation(), back.Orientation()) && errDeg > worst {
					worst = errDeg
					worstCase = fmt.Sprintf("%.2f deg from %s toward azimuth %.0f deg comes back %.3f deg off",
						tilt, map[bool]string{false: "+Z", true: "-Z"}[down], az, errDeg)
				}
			}
		}
	}
	test.That(t, worstCase, test.ShouldBeEmpty)
}
