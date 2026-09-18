package referenceframe

import (
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// spinDegs rotates about the Z axis; tiltDegs leans the Z axis over toward X.
func spinDegs(degs float64) spatialmath.Orientation {
	return &spatialmath.EulerAngles{Yaw: utils.DegToRad(degs)}
}

func tiltDegs(degs float64) spatialmath.Orientation {
	return &spatialmath.EulerAngles{Pitch: utils.DegToRad(degs)}
}

func composeOrientations(a, b spatialmath.Orientation) spatialmath.Orientation {
	return spatialmath.Compose(spatialmath.NewPoseFromOrientation(a), spatialmath.NewPoseFromOrientation(b)).Orientation()
}

func TestOrientationCloud(t *testing.T) {
	zero := spatialmath.NewZeroOrientation()
	// An upright cup: the Z axis may lean ~10 degrees, the spin about it is free.
	cup := &OrientationCloud{OX: 0.17, OY: 0.17, OZ: 0.015, Theta: 180}

	t.Run("spin is free, tilt is bounded", func(t *testing.T) {
		for _, degs := range []float64{0, 45, 90, 179, -120} {
			test.That(t, cup.OrientationInCloud(zero, spinDegs(degs)), test.ShouldBeTrue)
			test.That(t, cup.Excess(zero, spinDegs(degs)), test.ShouldEqual, 0)
		}
		test.That(t, cup.OrientationInCloud(zero, tiltDegs(5)), test.ShouldBeTrue)
		test.That(t, cup.Excess(zero, tiltDegs(5)), test.ShouldEqual, 0)
		test.That(t, cup.OrientationInCloud(zero, tiltDegs(20)), test.ShouldBeFalse)
		// Excess grows with the violation and is continuous at the boundary.
		test.That(t, cup.Excess(zero, tiltDegs(20)), test.ShouldBeGreaterThan, cup.Excess(zero, tiltDegs(15)))
		test.That(t, cup.Excess(zero, tiltDegs(15)), test.ShouldBeGreaterThan, 0)
		test.That(t, cup.Excess(zero, tiltDegs(9.9)), test.ShouldBeLessThan, 0.5)
	})

	t.Run("measured in the reference frame", func(t *testing.T) {
		// A spin about the reference's own Z axis stays in the cloud however the reference is
		// tilted; the same spin about the world Z axis leans the reference's axis over.
		ref := tiltDegs(30)
		test.That(t, cup.OrientationInCloud(ref, ref), test.ShouldBeTrue)
		test.That(t, cup.OrientationInCloud(ref, composeOrientations(ref, spinDegs(90))), test.ShouldBeTrue)
		test.That(t, cup.OrientationInCloud(ref, composeOrientations(spinDegs(90), ref)), test.ShouldBeFalse)
	})

	t.Run("agrees with PoseCloud", func(t *testing.T) {
		pc := &PoseCloud{OX: cup.OX, OY: cup.OY, OZ: cup.OZ, Theta: cup.Theta}
		ref := tiltDegs(30)
		for _, cand := range []spatialmath.Orientation{
			zero, spinDegs(90), tiltDegs(5), tiltDegs(20), ref,
			composeOrientations(ref, spinDegs(90)), composeOrientations(spinDegs(90), ref),
		} {
			for _, r := range []spatialmath.Orientation{zero, ref} {
				test.That(t, cup.OrientationInCloud(r, cand), test.ShouldEqual,
					pc.PoseInCloud(spatialmath.NewPoseFromOrientation(r), spatialmath.NewPoseFromOrientation(cand)))
			}
		}
	})

	t.Run("theta leeway", func(t *testing.T) {
		th := &OrientationCloud{Theta: 15}
		test.That(t, th.OrientationInCloud(zero, spinDegs(10)), test.ShouldBeTrue)
		test.That(t, th.OrientationInCloud(zero, spinDegs(20)), test.ShouldBeFalse)
		test.That(t, th.Excess(zero, spinDegs(20)), test.ShouldAlmostEqual, 5, 1e-6)
	})

	t.Run("zero cloud admits only the reference", func(t *testing.T) {
		none := &OrientationCloud{}
		test.That(t, none.OrientationInCloud(zero, zero), test.ShouldBeTrue)
		test.That(t, none.Excess(zero, zero), test.ShouldEqual, 0)
		test.That(t, none.OrientationInCloud(zero, spinDegs(1)), test.ShouldBeFalse)
		test.That(t, none.OrientationInCloud(zero, tiltDegs(1)), test.ShouldBeFalse)
	})
}

func TestOrientationCloudInscribedAngle(t *testing.T) {
	cup := OrientationCloud{OX: 0.17, OY: 0.17, OZ: 0.015, Theta: 180}
	test.That(t, cup.InscribedAngleDegs(), test.ShouldAlmostEqual, utils.RadToDeg(math.Asin(0.17)), 1e-9)
	test.That(t, (&OrientationCloud{OX: 1, OY: 1, OZ: 0.5, Theta: 180}).InscribedAngleDegs(), test.ShouldAlmostEqual, 60, 1e-9)
	test.That(t, (&OrientationCloud{Theta: 180}).InscribedAngleDegs(), test.ShouldEqual, 0)
	test.That(t, (&OrientationCloud{}).InscribedAngleDegs(), test.ShouldEqual, 0)
	// Any bound on theta rules out a ball: past the pole, theta reads as the azimuth of the lean,
	// so a small lean toward -X already reads as 180 degrees and a small roll as 90.
	test.That(t, (&OrientationCloud{OX: 1, OY: 1, OZ: 2, Theta: 179}).InscribedAngleDegs(), test.ShouldEqual, 0)
	leanBack := &spatialmath.R4AA{Theta: utils.DegToRad(2), RX: 0, RY: -1, RZ: 0}
	test.That(t, (&OrientationCloud{OX: 1, OY: 1, OZ: 2, Theta: 179}).OrientationInCloud(spatialmath.NewZeroOrientation(), leanBack),
		test.ShouldBeFalse)
	rolled := &spatialmath.EulerAngles{Roll: utils.DegToRad(2)}
	test.That(t, (&OrientationCloud{OX: 1, OY: 1, OZ: 2, Theta: 89}).OrientationInCloud(spatialmath.NewZeroOrientation(), rolled),
		test.ShouldBeFalse)

	// Every rotation by less than the inscribed angle, about any axis, lands in the cloud.
	zero := spatialmath.NewZeroOrientation()
	rng := rand.New(rand.NewSource(1))
	for _, cloud := range []OrientationCloud{
		cup,
		{OX: 1, OY: 1, OZ: 0.5, Theta: 180},
		{OX: 0.5, OY: 0.5, OZ: 0.2, Theta: 180},
		{OX: 0.1, OY: 0.9, OZ: 1, Theta: 180},
	} {
		inscribed := cloud.InscribedAngleDegs()
		test.That(t, inscribed, test.ShouldBeGreaterThan, 0)
		for range 2000 {
			axis := r3.Vector{X: rng.NormFloat64(), Y: rng.NormFloat64(), Z: rng.NormFloat64()}.Normalize()
			angle := utils.DegToRad(inscribed * rng.Float64())
			cand := &spatialmath.R4AA{Theta: angle, RX: axis.X, RY: axis.Y, RZ: axis.Z}
			test.That(t, cloud.OrientationInCloud(zero, cand), test.ShouldBeTrue)
		}
	}
}
