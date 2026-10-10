package spatialmath

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/utils"
)

// Regression tests for https://github.com/viamrobotics/rdk/issues/6627. The pole radius is applied to 1-|OZ|, which is about
// half the tilt squared, so it reaches acos(1-1e-4) = 0.81 degrees from +/-Z. Orientations in that zone used to decode as if
// they leaned toward +X, whatever OX/OY said, and came back from an API round trip up to 2x the tilt off.

var nearPoleTiltsDeg = []float64{1e-4, 0.05, 0.2, 0.5, 0.8, 0.81, 0.82, 2}

func nearPoleName(down bool, tiltDeg, azDeg, thetaDeg float64) string {
	pole := "+Z"
	if down {
		pole = "-Z"
	}
	return fmt.Sprintf("tilted %g deg from %s toward azimuth %g deg, theta %g deg", tiltDeg, pole, azDeg, thetaDeg)
}

// nearPoleOV points tiltDeg away from +Z (or -Z when down), leaning toward azDeg in the XY plane.
func nearPoleOV(down bool, tiltDeg, azDeg, thetaDeg float64) *OrientationVectorDegrees {
	tilt, az := utils.DegToRad(tiltDeg), utils.DegToRad(azDeg)
	z := math.Cos(tilt)
	if down {
		z = -z
	}
	return &OrientationVectorDegrees{OX: math.Sin(tilt) * math.Cos(az), OY: math.Sin(tilt) * math.Sin(az), OZ: z, Theta: thetaDeg}
}

// nearPoleOrientation is Rz(az) Ry(tilt) Rz(twist), with the tilt taken from -Z when down.
func nearPoleOrientation(down bool, tiltDeg, azDeg, twistDeg float64) Orientation {
	lean := utils.DegToRad(tiltDeg)
	if down {
		lean = math.Pi - lean
	}
	return Compose(
		NewPoseFromOrientation(&R4AA{Theta: utils.DegToRad(azDeg), RZ: 1}),
		Compose(
			NewPoseFromOrientation(&R4AA{Theta: lean, RY: 1}),
			NewPoseFromOrientation(&R4AA{Theta: utils.DegToRad(twistDeg), RZ: 1}),
		),
	).Orientation()
}

func angleBetweenDeg(a, b Orientation) float64 {
	return utils.RadToDeg(math.Abs(OrientationBetween(a, b).AxisAngles().Theta))
}

// The decoded +Z axis must point along (OX, OY, OZ), whatever theta is, on every path an orientation vector is decoded by.
func TestOrientationVectorPointsAlongXYZNearPoles(t *testing.T) {
	decoders := map[string]func(t *testing.T, ov *OrientationVectorDegrees) Orientation{
		"OrientationVectorDegrees": func(t *testing.T, ov *OrientationVectorDegrees) Orientation { return ov },
		"NewPoseFromProtobuf": func(t *testing.T, ov *OrientationVectorDegrees) Orientation {
			return NewPoseFromProtobuf(&commonpb.Pose{OX: ov.OX, OY: ov.OY, OZ: ov.OZ, Theta: ov.Theta}).Orientation()
		},
		"OrientationConfig": func(t *testing.T, ov *OrientationVectorDegrees) Orientation {
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
			for _, down := range []bool{false, true} {
				for _, tilt := range nearPoleTiltsDeg {
					for az := 0.; az < 360; az += 45 {
						for _, theta := range []float64{0, 90, -170} {
							ov := nearPoleOV(down, tilt, az, theta)
							want := r3.Vector{X: ov.OX, Y: ov.OY, Z: ov.OZ}.Normalize()
							got := QuatToRotationMatrix(decode(t, ov).Quaternion()).Col(2)
							if errDeg := utils.RadToDeg(float64(got.Angle(want))); errDeg > 1e-6 {
								t.Fatalf("%s: +Z points %g deg off", nearPoleName(down, tilt, az, theta), errDeg)
							}
						}
					}
				}
			}
		})
	}
}

// Every pose sent over the API goes through PoseToProtobuf and NewPoseFromProtobuf.
func TestPoseProtobufRoundTripNearPoles(t *testing.T) {
	for _, down := range []bool{false, true} {
		for _, tilt := range nearPoleTiltsDeg {
			for az := 0.; az < 360; az += 30 {
				for _, twist := range []float64{0, 30, -70, 180} {
					o := nearPoleOrientation(down, tilt, az, twist)
					back := NewPoseFromProtobuf(PoseToProtobuf(NewPoseFromOrientation(o))).Orientation()
					if errDeg := angleBetweenDeg(o, back); errDeg > 1e-5 {
						t.Fatalf("%s: comes back %g deg off", nearPoleName(down, tilt, az, twist), errDeg)
					}
				}
			}
		}
	}
}

// TestOrientationVectorPoleRadius leaning the other way, away from the +X the old decoder assumed.
func TestOrientationVectorPoleRadiusMirrored(t *testing.T) {
	for _, ov := range []*OrientationVectorDegrees{
		{Theta: 90.2029644505, OX: 0.0050164674, OY: 0.0079070413, OZ: 0.9999561559},
		{Theta: 90.2029644505, OX: -0.0050164674, OY: -0.0079070413, OZ: 0.9999561559},
		{Theta: -89.2515361355, OX: 0.0037393949, OY: -0.009106087, OZ: -0.9999515469},
	} {
		composed := Quaternion(ov.Quaternion()).OrientationVectorDegrees()
		test.That(t, composed.Theta, test.ShouldAlmostEqual, ov.Theta, 1e-9)
		test.That(t, composed.OX, test.ShouldAlmostEqual, ov.OX, 1e-9)
		test.That(t, composed.OY, test.ShouldAlmostEqual, ov.OY, 1e-9)
		test.That(t, composed.OZ, test.ShouldAlmostEqual, ov.OZ, 1e-9)
	}
}

// Peers that predate the fix decode pole-radius vectors as Ry(lat) Rz(theta), dropping OX/OY. Theta must keep the meaning they
// expect, so they are no worse off than before (2x the tilt at most), and their encodings must decode better than they used to.
func TestOrientationVectorNearPolesOldPeers(t *testing.T) {
	oldDecode := func(ov *OrientationVector) Orientation {
		ov.Normalize()
		q := mgl64.AnglesToQuat(0, math.Acos(ov.OZ), ov.Theta, mgl64.ZYZ)
		return &Quaternion{Real: q.W, Imag: q.X(), Jmag: q.Y(), Kmag: q.Z()}
	}
	for _, down := range []bool{false, true} {
		for _, tilt := range nearPoleTiltsDeg {
			if tilt > 0.81 {
				continue
			}
			for az := 0.; az < 360; az += 30 {
				for _, twist := range []float64{0, 30, -70, 180} {
					o := nearPoleOrientation(down, tilt, az, twist)
					ov := QuatToOV(o.Quaternion())
					test.That(t, angleBetweenDeg(o, oldDecode(ov)), test.ShouldBeLessThanOrEqualTo, 2*tilt*1.001+1e-9)

					// An old encoder's theta is the heading of the local -X axis, exact only on the pole itself.
					newX := QuatToRotationMatrix(o.Quaternion()).Col(0).Mul(-1)
					oldTheta := -math.Atan2(newX.Y, -newX.X)
					if down {
						oldTheta = -math.Atan2(newX.Y, newX.X)
					}
					fromOld := &OrientationVector{OX: ov.OX, OY: ov.OY, OZ: ov.OZ, Theta: oldTheta}
					test.That(t, angleBetweenDeg(o, fromOld), test.ShouldBeLessThan, 0.01)
				}
			}
		}
	}
}

func TestOrientationVectorExactPolesUnchanged(t *testing.T) {
	for _, tc := range []struct {
		q     quat.Number
		theta float64
		oz    float64
	}{
		{quat.Number{Real: 1}, 0, 1},
		{quat.Number{Real: math.Cos(0.5), Kmag: math.Sin(0.5)}, 1, 1},
		{quat.Number{Jmag: 1}, 0, -1},
		{quat.Number{Imag: 1}, -math.Pi, -1},
	} {
		ov := QuatToOV(tc.q)
		test.That(t, ov.Theta, test.ShouldAlmostEqual, tc.theta, 1e-12)
		test.That(t, ov.OZ, test.ShouldAlmostEqual, tc.oz, 1e-12)
		test.That(t, QuatToOV(ov.Quaternion()).Theta, test.ShouldAlmostEqual, tc.theta, 1e-12)
	}
}

// The encoder and decoder each decide whether a vector is within the pole radius, and theta means something different on each
// side. Straddle the edge of the pole radius ulp by ulp to check they always agree.
func TestPoseProtobufRoundTripAtPoleRadiusEdge(t *testing.T) {
	edge := math.Acos(1 - orientationVectorPoleRadius)
	for _, down := range []bool{false, true} {
		for k := -500; k <= 500; k++ {
			lean := edge + float64(k)*1e-16
			if down {
				lean = math.Pi - lean
			}
			for az := 7.5; az < 360; az += 15 {
				for _, twist := range []float64{0, 50, -120} {
					o := Compose(
						NewPoseFromOrientation(&R4AA{Theta: utils.DegToRad(az), RZ: 1}),
						Compose(
							NewPoseFromOrientation(&R4AA{Theta: lean, RY: 1}),
							NewPoseFromOrientation(&R4AA{Theta: utils.DegToRad(twist), RZ: 1}),
						),
					).Orientation()
					back := NewPoseFromProtobuf(PoseToProtobuf(NewPoseFromOrientation(o))).Orientation()
					if errDeg := angleBetweenDeg(o, back); errDeg > 1e-9 {
						t.Fatalf("%de-16 rad past the pole radius edge, %s: comes back %g deg off",
							k, nearPoleName(down, utils.RadToDeg(edge), az, twist), errDeg)
					}
				}
			}
		}
	}
}

// Re-encoding a decoded orientation vector must give back the same vector, so configs and stored poses don't drift.
func TestOrientationVectorReencodeIsStable(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		q := quat.Number{Real: r.NormFloat64(), Imag: r.NormFloat64(), Jmag: r.NormFloat64(), Kmag: r.NormFloat64()}
		if i%2 == 0 {
			// Shrink the X/Y rotation so half the samples land near the poles, down to 1e-8 rad away.
			s := math.Pow(10, -8*r.Float64())
			q.Imag *= s
			q.Jmag *= s
		}
		q = Normalize(q)
		ov := QuatToOV(q)
		again := QuatToOV((&OrientationVector{Theta: ov.Theta, OX: ov.OX, OY: ov.OY, OZ: ov.OZ}).Quaternion())
		// acos(OZ) in Quaternion() resolves tilts only to ~1e-8 rad, which bounds how well OX/OY survive.
		test.That(t, again.OX, test.ShouldAlmostEqual, ov.OX, 1e-7)
		test.That(t, again.OY, test.ShouldAlmostEqual, ov.OY, 1e-7)
		test.That(t, again.OZ, test.ShouldAlmostEqual, ov.OZ, 1e-7)
		test.That(t, math.Remainder(again.Theta-ov.Theta, 2*math.Pi), test.ShouldAlmostEqual, 0, 1e-9)
	}
}

// Signed zeros and unnormalized vectors, both common in hand-written configs, decode the same as their plain forms.
func TestOrientationVectorNearPolesInputForms(t *testing.T) {
	negZero := math.Copysign(0, -1)
	for _, oz := range []float64{1, -1} {
		plain := &OrientationVector{Theta: 1, OZ: oz}
		for _, v := range [][2]float64{{negZero, 0}, {0, negZero}, {negZero, negZero}} {
			signed := &OrientationVector{Theta: 1, OX: v[0], OY: v[1], OZ: oz}
			test.That(t, angleBetweenDeg(plain, signed), test.ShouldBeLessThan, 1e-9)
		}
		unit := &OrientationVector{Theta: 1, OX: 0.003, OY: -0.004, OZ: oz}
		scaled := &OrientationVector{Theta: 1, OX: 0.003 * 7, OY: -0.004 * 7, OZ: oz * 7}
		test.That(t, angleBetweenDeg(unit, scaled), test.ShouldBeLessThan, 1e-9)
	}
}
