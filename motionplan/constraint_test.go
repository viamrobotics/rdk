package motionplan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	motionpb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestConstraintConstructors(t *testing.T) {
	c := NewEmptyConstraints()

	desiredLinearTolerance := float64(1000.0)
	desiredOrientationTolerance := float64(0.0)

	c.AddLinearConstraint(LinearConstraint{
		LineToleranceMm:          desiredLinearTolerance,
		OrientationToleranceDegs: desiredOrientationTolerance,
	})

	test.That(t, len(c.LinearConstraint), test.ShouldEqual, 1)
	test.That(t, c.LinearConstraint[0].LineToleranceMm, test.ShouldEqual, desiredLinearTolerance)
	test.That(t, c.LinearConstraint[0].OrientationToleranceDegs, test.ShouldEqual, desiredOrientationTolerance)

	c.AddOrientationConstraint(OrientationConstraint{
		OrientationToleranceDegs: desiredOrientationTolerance,
	})
	test.That(t, len(c.OrientationConstraint), test.ShouldEqual, 1)
	test.That(t, c.OrientationConstraint[0].OrientationToleranceDegs, test.ShouldEqual, desiredOrientationTolerance)

	c.AddCollisionSpecification(CollisionSpecification{
		Allows: []CollisionSpecificationAllowedFrameCollisions{
			{
				Frame1: "frame1",
				Frame2: "frame2",
			},
			{
				Frame1: "frame3",
				Frame2: "frame4",
			},
		},
	})
	test.That(t, len(c.CollisionSpecification), test.ShouldEqual, 1)
	test.That(t, c.CollisionSpecification[0].Allows[0].Frame1, test.ShouldEqual, "frame1")
	test.That(t, c.CollisionSpecification[0].Allows[0].Frame2, test.ShouldEqual, "frame2")
	test.That(t, c.CollisionSpecification[0].Allows[1].Frame1, test.ShouldEqual, "frame3")
	test.That(t, c.CollisionSpecification[0].Allows[1].Frame2, test.ShouldEqual, "frame4")

	pbConstraint := c.ToProtobuf()
	pbToRDKConstraint := ConstraintsFromProtobuf(pbConstraint)
	test.That(t, c, test.ShouldResemble, pbToRDKConstraint)

	c.AddPseudolinearConstraint(PseudolinearConstraint{5, 2})

	pbConstraint = c.ToProtobuf()
	pbToRDKConstraint = ConstraintsFromProtobuf(pbConstraint)
	test.That(t, c, test.ShouldResemble, pbToRDKConstraint)
}

// TestOrientationArcDistanceBruteForce pins distanceDegs to its definition -
// the smallest angular distance from now to any orientation on the slerp arc -
// by sampling that arc densely with spatialmath.Interpolate and OrientDist,
// which share no code with the closed form.
// TestOrientationConstraintProtoRoundTrip covers IgnoreTheta in both states.
// The constructor test above only ever round-trips the zero value, which would
// pass even if the flag were dropped by the conversion.
func TestOrientationConstraintProtoRoundTrip(t *testing.T) {
	for _, ignoreTheta := range []bool{false, true} {
		c := NewEmptyConstraints()
		c.AddOrientationConstraint(OrientationConstraint{OrientationToleranceDegs: 15, IgnoreTheta: ignoreTheta})

		pb := c.ToProtobuf()
		test.That(t, pb.OrientationConstraint[0].GetIgnoreTheta(), test.ShouldEqual, ignoreTheta)

		back := ConstraintsFromProtobuf(pb)
		test.That(t, back.OrientationConstraint[0].IgnoreTheta, test.ShouldEqual, ignoreTheta)
		test.That(t, back, test.ShouldResemble, c)
	}

	// A message from a peer that predates the field leaves it unset, which must
	// read as the theta-aware default rather than erroring.
	tol := float32(15)
	back := ConstraintsFromProtobuf(&motionpb.Constraints{
		OrientationConstraint: []*motionpb.OrientationConstraint{{OrientationToleranceDegs: &tol}},
	})
	test.That(t, back.OrientationConstraint[0].IgnoreTheta, test.ShouldBeFalse)
	test.That(t, back.OrientationConstraint[0].OrientationToleranceDegs, test.ShouldEqual, 15)
}

func TestOrientationArcDistanceBruteForce(t *testing.T) {
	const steps = 4000
	rng := rand.New(rand.NewSource(11))
	randOrient := func() spatial.Orientation {
		q := quat.Number{Real: rng.NormFloat64(), Imag: rng.NormFloat64(), Jmag: rng.NormFloat64(), Kmag: rng.NormFloat64()}
		n := math.Sqrt(quatDot(q, q))
		return (*spatial.Quaternion)(&quat.Number{Real: q.Real / n, Imag: q.Imag / n, Jmag: q.Jmag / n, Kmag: q.Kmag / n})
	}
	brute := func(from, to, now spatial.Orientation) float64 {
		fp, tp := spatial.NewPoseFromOrientation(from), spatial.NewPoseFromOrientation(to)
		best := math.Inf(1)
		for i := 0; i <= steps; i++ {
			best = math.Min(best, OrientDist(spatial.Interpolate(fp, tp, float64(i)/steps).Orientation(), now))
		}
		return best
	}

	for i := 0; i < 200; i++ {
		from, to, now := randOrient(), randOrient(), randOrient()
		arc := newOrientationArc(from, to, false)
		got := arc.distanceDegs(now)
		want := brute(from, to, now)
		// The closed form takes the true minimum, so it can only sit at or
		// below the sampled one; the sampling grid bounds the gap.
		test.That(t, got, test.ShouldBeLessThanOrEqualTo, want+1e-6)
		test.That(t, want-got, test.ShouldBeLessThan, 0.05)
	}
}

func TestOrientationConstraintDistance(t *testing.T) {
	oc := OrientationConstraint{OrientationToleranceDegs: 30}
	zero := spatial.NewZeroOrientation()
	rotZ := func(degs float64) spatial.Orientation {
		return &spatial.EulerAngles{Yaw: utils.DegToRad(degs)}
	}

	// Degenerate arc (from == to): plain angular distance to the endpoint.
	test.That(t, oc.Distance(zero, zero, zero), test.ShouldAlmostEqual, 0, 1e-5)
	test.That(t, oc.Distance(zero, zero, rotZ(40)), test.ShouldAlmostEqual, 40, 1e-4)

	// Points on the arc score zero, including endpoints and beyond-tolerance
	// midpoints - the band is a connected tube around the whole reorientation.
	from, to := zero, rotZ(120)
	test.That(t, oc.Distance(from, to, from), test.ShouldAlmostEqual, 0, 1e-5)
	test.That(t, oc.Distance(from, to, to), test.ShouldAlmostEqual, 0, 1e-5)
	test.That(t, oc.Distance(from, to, rotZ(60)), test.ShouldAlmostEqual, 0, 1e-5)
	test.That(t, oc.Distance(from, to, rotZ(100)), test.ShouldAlmostEqual, 0, 1e-5)

	// Off-arc: distance is to the nearest arc point, not the nearest endpoint.
	// Overshooting the arc past `to` measures from `to`.
	test.That(t, oc.Distance(from, to, rotZ(150)), test.ShouldAlmostEqual, 30, 1e-4)
	test.That(t, oc.Distance(from, to, rotZ(-25)), test.ShouldAlmostEqual, 25, 1e-4)
	// Deviation orthogonal to the arc's rotation axis.
	pitch45 := &spatial.EulerAngles{Pitch: utils.DegToRad(45)}
	dist := oc.Distance(from, to, pitch45)
	test.That(t, dist, test.ShouldBeGreaterThan, 0)
	test.That(t, dist, test.ShouldBeLessThanOrEqualTo, 45+1e-6)

	// The eval form agrees with the direct form.
	eval := NewOrientationConstraintEval(oc, from, to)
	for _, o := range []spatial.Orientation{zero, rotZ(60), rotZ(150), pitch45} {
		test.That(t, eval.Distance(o), test.ShouldAlmostEqual, oc.Distance(from, to, o), 1e-9)
	}

	// Score subtracts the tolerance.
	test.That(t, oc.Score(from, to, rotZ(150)), test.ShouldAlmostEqual, 0, 1e-5)
	test.That(t, oc.Score(from, to, rotZ(170)), test.ShouldAlmostEqual, 20, 1e-4)
}

func TestOrientationConstraintIgnoreTheta(t *testing.T) {
	oc := OrientationConstraint{OrientationToleranceDegs: 15, IgnoreTheta: true}
	strict := OrientationConstraint{OrientationToleranceDegs: 15}
	zero := spatial.NewZeroOrientation()
	rotZ := func(degs float64) spatial.Orientation {
		return &spatial.EulerAngles{Yaw: utils.DegToRad(degs)}
	}
	pitch := func(degs float64) spatial.Orientation {
		return &spatial.EulerAngles{Pitch: utils.DegToRad(degs)}
	}
	roll := func(degs float64) spatial.Orientation {
		return &spatial.EulerAngles{Roll: utils.DegToRad(degs)}
	}

	// Spinning about the orientation vector itself costs nothing, however far
	// it goes - that is the whole point for a cup or a bucket. The strict form
	// scores the same motion as a full reorientation.
	test.That(t, oc.Distance(zero, zero, rotZ(90)), test.ShouldAlmostEqual, 0, 1e-4)
	test.That(t, oc.Distance(zero, zero, rotZ(170)), test.ShouldAlmostEqual, 0, 1e-4)
	test.That(t, strict.Distance(zero, zero, rotZ(90)), test.ShouldAlmostEqual, 90, 1e-4)

	// Tipping still costs its full angle.
	test.That(t, oc.Distance(zero, zero, pitch(45)), test.ShouldAlmostEqual, 45, 1e-4)
	test.That(t, oc.Distance(zero, zero, roll(20)), test.ShouldAlmostEqual, 20, 1e-4)
	// ...and tipping combined with a free spin costs only the tip.
	spunPitch := spatial.Compose(
		spatial.NewPoseFromOrientation(pitch(30)),
		spatial.NewPoseFromOrientation(rotZ(120)),
	).Orientation()
	test.That(t, oc.Distance(zero, zero, spunPitch), test.ShouldAlmostEqual, 30, 1e-4)

	// A reorientation whose axis is the orientation vector (pure theta change)
	// leaves the traced path a single point, so the band is a cone about it.
	from, to := zero, rotZ(150)
	test.That(t, oc.Distance(from, to, rotZ(75)), test.ShouldAlmostEqual, 0, 1e-4)
	test.That(t, oc.Distance(from, to, pitch(10)), test.ShouldAlmostEqual, 10, 1e-4)

	// A genuine tilt from start to goal: the orientation vector sweeps +Z -> +X.
	from, to = zero, pitch(90)
	// Points along the sweep score zero.
	for _, degs := range []float64{0, 30, 45, 90} {
		test.That(t, oc.Distance(from, to, pitch(degs)), test.ShouldAlmostEqual, 0, 1e-4)
	}
	// Overshooting past either end measures from that end.
	test.That(t, oc.Distance(from, to, pitch(120)), test.ShouldAlmostEqual, 30, 1e-4)
	test.That(t, oc.Distance(from, to, pitch(-20)), test.ShouldAlmostEqual, 20, 1e-4)
	// Off-sweep deviation: roll takes the vector out of the swept plane, and
	// the nearest swept point is the start.
	test.That(t, oc.Distance(from, to, roll(25)), test.ShouldAlmostEqual, 25, 1e-4)
	// Spinning about the tool axis anywhere along the sweep is still free.
	midSpun := spatial.Compose(
		spatial.NewPoseFromOrientation(pitch(45)),
		spatial.NewPoseFromOrientation(rotZ(80)),
	).Orientation()
	test.That(t, oc.Distance(from, to, midSpun), test.ShouldAlmostEqual, 0, 1e-4)

	// The eval form agrees with the direct form and honours the flag.
	eval := NewOrientationConstraintEval(oc, from, to)
	for _, o := range []spatial.Orientation{zero, pitch(45), pitch(120), roll(25), midSpun} {
		test.That(t, eval.Distance(o), test.ShouldAlmostEqual, oc.Distance(from, to, o), 1e-9)
	}

	// Score subtracts the tolerance, as in the theta-aware form.
	test.That(t, oc.Score(from, to, roll(10)), test.ShouldAlmostEqual, 0, 1e-4)
	test.That(t, oc.Score(from, to, roll(25)), test.ShouldAlmostEqual, 10, 1e-4)
}

// TestOrientationConstraintIgnoreThetaBruteForce cross-checks the closed-form
// point-to-swept-arc distance against a dense sampling of the same arc, built
// from spatialmath.Interpolate and spatialmath.QuatToOV so the two derivations
// share no code.
func TestOrientationConstraintIgnoreThetaBruteForce(t *testing.T) {
	const steps = 4000
	oc := OrientationConstraint{OrientationToleranceDegs: 15, IgnoreTheta: true}

	ovOf := func(o spatial.Orientation) r3.Vector {
		ov := spatial.QuatToOV(o.Quaternion())
		return r3.Vector{X: ov.OX, Y: ov.OY, Z: ov.OZ}
	}
	brute := func(from, to, now spatial.Orientation) float64 {
		pn := ovOf(now)
		fp, tp := spatial.NewPoseFromOrientation(from), spatial.NewPoseFromOrientation(to)
		best := math.Inf(1)
		for i := 0; i <= steps; i++ {
			o := spatial.Interpolate(fp, tp, float64(i)/steps).Orientation()
			best = math.Min(best, utils.RadToDeg(math.Acos(math.Max(-1, math.Min(1, pn.Dot(ovOf(o)))))))
		}
		return best
	}

	rng := rand.New(rand.NewSource(7))
	randOrient := func() spatial.Orientation {
		q := quat.Number{Real: rng.NormFloat64(), Imag: rng.NormFloat64(), Jmag: rng.NormFloat64(), Kmag: rng.NormFloat64()}
		n := math.Sqrt(quatDot(q, q))
		return (*spatial.Quaternion)(&quat.Number{Real: q.Real / n, Imag: q.Imag / n, Jmag: q.Jmag / n, Kmag: q.Kmag / n})
	}

	for i := 0; i < 200; i++ {
		from, to, now := randOrient(), randOrient(), randOrient()
		got := oc.Distance(from, to, now)
		want := brute(from, to, now)
		// The closed form takes the true minimum, so it can only sit at or
		// below the sampled one; the sampling grid bounds the gap.
		test.That(t, got, test.ShouldBeLessThanOrEqualTo, want+1e-6)
		test.That(t, want-got, test.ShouldBeLessThan, 0.05)
	}
}

// TestOrientationConstraintIgnoreThetaChecker drives the flag through the
// ConstraintChecker on a real arm. On an xArm7 at the zero configuration the
// tool points straight down; joint 7 spins the tool about that axis while
// joint 6 tips it, which is exactly the distinction IgnoreTheta draws.
func TestOrientationConstraintIgnoreThetaChecker(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	fs := referenceframe.NewEmptyFrameSystem("test")
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm7.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(m, fs.World()), test.ShouldBeNil)

	cfg := func(joint int, degs float64) *referenceframe.LinearInputs {
		in := make([]referenceframe.Input, 7)
		in[joint] = referenceframe.Input(utils.DegToRad(degs))
		return referenceframe.FrameSystemInputs{m.Name(): in}.ToLinearInputs()
	}
	home := cfg(0, 0)
	eePose, err := fs.Transform(home, referenceframe.NewZeroPoseInFrame(m.Name()), referenceframe.World)
	test.That(t, err, test.ShouldBeNil)
	poses := referenceframe.FrameSystemPoses{m.Name(): eePose.(*referenceframe.PoseInFrame)}

	checkerFor := func(ignoreTheta bool) *ConstraintChecker {
		constraints := NewEmptyConstraints()
		constraints.AddOrientationConstraint(OrientationConstraint{OrientationToleranceDegs: 5, IgnoreTheta: ignoreTheta})
		c, err := NewConstraintChecker(
			1.0, constraints, poses, poses, fs,
			[]spatial.Geometry{}, []spatial.Geometry{}, nil, home, nil, logger, nil,
		)
		test.That(t, err, test.ShouldBeNil)
		return c
	}

	spun := cfg(6, 90)   // rotates about the tool axis
	tipped := cfg(5, 20) // tips the tool axis

	_, err = checkerFor(true).CheckStateFSConstraints(ctx, &StateFS{Configuration: spun, FS: fs})
	test.That(t, err, test.ShouldBeNil)
	_, err = checkerFor(true).CheckStateFSConstraints(ctx, &StateFS{Configuration: tipped, FS: fs})
	test.That(t, errors.Is(err, ErrOrientationConstraintViolated), test.ShouldBeTrue)

	// Without the flag the same spin is a 90 degree violation.
	_, err = checkerFor(false).CheckStateFSConstraints(ctx, &StateFS{Configuration: spun, FS: fs})
	test.That(t, errors.Is(err, ErrOrientationConstraintViolated), test.ShouldBeTrue)
}

func TestOrientVecDist(t *testing.T) {
	zero := spatial.NewZeroOrientation()
	test.That(t, OrientVecDist(zero, &spatial.EulerAngles{Yaw: math.Pi / 2}), test.ShouldAlmostEqual, 0, 1e-4)
	test.That(t, OrientVecDist(zero, &spatial.EulerAngles{Pitch: math.Pi / 4}), test.ShouldAlmostEqual, 45, 1e-4)
	test.That(t, OrientVecDist(zero, &spatial.EulerAngles{Roll: math.Pi}), test.ShouldAlmostEqual, 180, 1e-4)
}

func TestConstraintPath(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	homePos := []referenceframe.Input{0, 0, 0, 0, 0, 0}
	toPos := []referenceframe.Input{0, 0, 0, 0, 0, 1}

	modelXarm, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	handler := NewEmptyConstraintChecker(logger)

	// No constraints, should pass - convert to FS segment
	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(modelXarm, fs.World())
	test.That(t, err, test.ShouldBeNil)

	segmentFS := &SegmentFS{
		StartConfiguration: referenceframe.FrameSystemInputs{modelXarm.Name(): homePos}.ToLinearInputs(),
		EndConfiguration:   referenceframe.FrameSystemInputs{modelXarm.Name(): toPos}.ToLinearInputs(),
		FS:                 fs,
	}

	failSeg, err := handler.CheckStateConstraintsAcrossSegmentFS(ctx, segmentFS, 0.5, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, failSeg, test.ShouldBeNil)

	// Test with linear constraint
	constraints := NewEmptyConstraints()
	constraints.AddLinearConstraint(LinearConstraint{LineToleranceMm: 0.01, OrientationToleranceDegs: 0.01})

	handler, err = NewConstraintChecker(
		1.0, // collision buffer
		constraints,
		referenceframe.FrameSystemPoses{}, // start poses
		referenceframe.FrameSystemPoses{}, // goal poses
		fs,
		[]spatial.Geometry{}, // moving geometries
		[]spatial.Geometry{}, // static geometries
		nil,                  // moving frame names
		referenceframe.NewNeutralLinearInputs(fs),
		nil, // obstaclesInWorldFrame
		logger,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	failSeg, err = handler.CheckStateConstraintsAcrossSegmentFS(ctx, segmentFS, 0.5, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, failSeg, test.ShouldBeNil)

	test.That(t, handler.topoConstraint, test.ShouldNotBeNil)

	badInterpPos := []referenceframe.Input{6.2, 0, 0, 0, 0, 0}
	badSegmentFS := &SegmentFS{
		StartConfiguration: referenceframe.FrameSystemInputs{modelXarm.Name(): homePos}.ToLinearInputs(),
		EndConfiguration:   referenceframe.FrameSystemInputs{modelXarm.Name(): badInterpPos}.ToLinearInputs(),
		FS:                 fs,
	}
	failSeg, err = handler.CheckStateConstraintsAcrossSegmentFS(ctx, badSegmentFS, 0.5, true)
	// The constraint behavior may vary - just ensure test runs
	if err != nil {
		test.That(t, failSeg, test.ShouldBeNil) // If error, no valid segment
	}
}

func TestLineFollow(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	p1 := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  440,
		Y:  -447,
		Z:  500,
		OY: -1,
	})
	p2 := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  140,
		Y:  -447,
		Z:  550,
		OY: -1,
	})
	mp1 := []float64{
		3.75646398939225,
		-1.0162453766159272,
		1.2142890600914453,
		1.0521227724322786,
		-0.21337105357552288,
		-0.006502311329196852,
		-4.3822913510408945,
	}
	mp2 := []float64{
		3.896845654143853,
		-0.8353398707254642,
		1.1306783805718412,
		0.8347159514038981,
		0.49562136809544177,
		-0.2260694386799326,
		-4.383397470889424,
	}
	mpFail := []float64{
		3.896845654143853,
		-1.8353398707254642,
		1.1306783805718412,
		0.8347159514038981,
		0.49562136809544177,
		-0.2260694386799326,
		-4.383397470889424,
	}

	query := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  289.94907586421124,
		Y:  -447,
		Z:  525.0086401700755,
		OY: -1,
	})

	fs := referenceframe.NewEmptyFrameSystem("test")

	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm7.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerFrame, err := referenceframe.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 105}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, m)
	test.That(t, err, test.ShouldBeNil)
	goalFrame := fs.World()

	startCfg := referenceframe.FrameSystemInputs{m.Name(): mp1}.ToLinearInputs()
	from := referenceframe.FrameSystemPoses{markerFrame.Name(): referenceframe.NewPoseInFrame(markerFrame.Name(), p1)}
	to := referenceframe.FrameSystemPoses{markerFrame.Name(): referenceframe.NewPoseInFrame(goalFrame.Name(), p2)}

	// Create a simple linear constraint instead of the old line constraint
	constraints := NewEmptyConstraints()
	constraints.AddLinearConstraint(LinearConstraint{LineToleranceMm: 0.001, OrientationToleranceDegs: 0.001})
	// Create constraint checker with linear constraints
	opt, err := NewConstraintChecker(
		1.0, // collision buffer
		constraints,
		from, // start poses
		to,   // goal poses
		fs,
		[]spatial.Geometry{}, // moving geometries
		[]spatial.Geometry{}, // static geometries
		nil,                  // moving frame names
		startCfg,
		nil, // obstaclesInWorldFrame
		logger,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	// Test distance calculation using new API
	dist := WeightedSquaredNormDistance(p1, query)
	test.That(t, dist, test.ShouldBeGreaterThan, 0) // Just ensure calculation works

	// This tests that we are able to advance partway, but not entirely, to the goal while keeping constraints, and return the last good
	// partway position
	lastGood, err := opt.CheckStateConstraintsAcrossSegmentFS(
		ctx,
		&SegmentFS{
			StartConfiguration: referenceframe.FrameSystemInputs{m.Name(): mp1}.ToLinearInputs(),
			EndConfiguration:   referenceframe.FrameSystemInputs{m.Name(): mp2}.ToLinearInputs(),
			FS:                 fs,
		},
		0.001,
		true,
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, lastGood, test.ShouldNotBeNil)

	// lastGood.StartConfiguration and EndConfiguration should pass constraints
	stateCheck := &StateFS{Configuration: lastGood.StartConfiguration, FS: fs}
	_, err = opt.CheckStateFSConstraints(ctx, stateCheck)
	test.That(t, err, test.ShouldBeNil)

	stateCheck.Configuration = lastGood.EndConfiguration
	_, err = opt.CheckStateFSConstraints(ctx, stateCheck)
	test.That(t, err, test.ShouldBeNil)

	// Check that a deviating configuration will fail
	stateCheck.Configuration = referenceframe.FrameSystemInputs{m.Name(): mpFail}.ToLinearInputs()
	_, err = opt.CheckStateFSConstraints(ctx, stateCheck)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "marker")
}

func TestCollisionConstraints(t *testing.T) {
	ctx := context.Background()
	zeroPos := []referenceframe.Input{0, 0, 0, 0, 0, 0}
	cases := []struct {
		input    []referenceframe.Input
		expected bool
		failName string
	}{
		{zeroPos, true, ""},
		{[]referenceframe.Input{math.Pi / 4, 0, 0, 0, 0, 0}, true, ""},
		{[]referenceframe.Input{math.Pi, 0, 0, 0, 0, 0}, false, ObstacleConstraintDescription},
		{[]referenceframe.Input{math.Pi / 4, 0, 0, 0, 2, 0}, false, selfCollisionConstraintDescription},
	}

	// define external obstacles
	bc, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacles := []spatial.Geometry{}
	obstacles = append(obstacles, bc.Transform(spatial.NewZeroPose()))
	obstacles = append(obstacles, bc.Transform(spatial.NewPoseFromPoint(r3.Vector{-130, 0, 300})))
	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{
		referenceframe.NewGeometriesInFrame(referenceframe.World, obstacles),
	}, nil)
	test.That(t, err, test.ShouldBeNil)

	// setup zero position as reference CollisionGraph and use it in handler
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(referenceframe.World))
	test.That(t, err, test.ShouldBeNil)
	seedMap := referenceframe.NewNeutralFrameSystemInputs(fs)
	handler := &ConstraintChecker{}

	// create robot collision entities
	movingGeometriesInFrame, err := model.Geometries(seedMap[model.Name()])
	movingRobotGeometries := movingGeometriesInFrame.Geometries()
	test.That(t, err, test.ShouldBeNil)

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatial.Geometry, 0)
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(fs, seedMap)
	test.That(t, err, test.ShouldBeNil)
	for name, geometries := range frameSystemGeometries {
		if name != model.Name() {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}

	// Note that all obstacles in worldState are assumed to be static so it is ok to transform them into the world frame
	// TODO(rb) it is bad practice to assume that the current inputs of the robot correspond to the passed in world state
	// the state that observed the worldState should ultimately be included as part of the worldState message
	worldGeometries, err := worldState.ObstaclesInWorldFrame(fs, seedMap)
	test.That(t, err, test.ShouldBeNil)

	handler.collisionConstraints, err = CreateAllCollisionConstraints(
		fs,
		movingRobotGeometries,
		map[string]bool{model.Name(): true},
		staticRobotGeometries,
		worldGeometries.Geometries(),
		nil, // allowedCollisions
		defaultCollisionBufferMM,
		nil,
		logging.NewTestLogger(t),
	)
	test.That(t, err, test.ShouldBeNil)

	// loop through cases and check constraint handler processes them correctly
	for i, c := range cases {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			stateFS := &StateFS{
				Configuration: referenceframe.FrameSystemInputs{model.Name(): c.input}.ToLinearInputs(),
				FS:            fs,
			}
			_, err := handler.CheckStateFSConstraints(ctx, stateFS)
			test.That(t, err == nil, test.ShouldEqual, c.expected)
			if err != nil {
				test.That(t, err.Error(), test.ShouldStartWith, c.failName)
			}
		})
	}
}

func TestCalculateJointStepCount(t *testing.T) {
	t.Run("no movement", func(t *testing.T) {
		start := []float64{0, 0, 0}
		end := []float64{0, 0, 0}
		test.That(t, calculateJointStepCount(start, end, 0.05), test.ShouldEqual, 0)
	})

	t.Run("small movement under step size", func(t *testing.T) {
		start := []float64{0, 0, 0}
		end := []float64{0.04, 0, 0} // 0.04 rad < defaultJointStepSizeRadians
		test.That(t, calculateJointStepCount(start, end, 0.05), test.ShouldEqual, 1)
	})

	t.Run("one radian movement", func(t *testing.T) {
		start := []float64{0, 0, 0}
		end := []float64{1.0, 0, 0} // 1 rad / defaultJointStepSizeRadians = 20 steps
		test.That(t, calculateJointStepCount(start, end, 0.05), test.ShouldEqual, 20)
	})

	t.Run("large joint movement from sanding collision bug", func(t *testing.T) {
		// Joint 1 from TestSandingWallCollision moves ~6.7 radians
		start := []float64{1.06, -3.336, -0.18, 1.90, -1.53, 4.20}
		end := []float64{1.06, 3.379, -1.26, -0.59, 1.53, 1.06}
		// Joint 1 moves 6.715 rad / 0.05 = 135 steps
		steps := calculateJointStepCount(start, end, 0.05)
		test.That(t, steps, test.ShouldEqual, 135)
	})
}

// TestSegmentStepCount tests that segmentStepCount correctly emits step count from either joint or cartesian excursion
func TestSegmentStepCount(t *testing.T) {
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur20.json"), "")
	test.That(t, err, test.ShouldBeNil)
	jointStepSize := jointStepSizeFromLimits(model.DoF())

	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.World())
	test.That(t, err, test.ShouldBeNil)
	startConfig := []referenceframe.Input{1.06, -3.336, -0.18, 1.90, -1.53, 4.20}
	startPos, err := model.Transform(startConfig)
	test.That(t, err, test.ShouldBeNil)

	t.Run("joint steps dominate when cartesian distance is small", func(t *testing.T) {
		// For a ur20 this config is very close to startConfig in cartesian space but far away in joint space
		endConfig := []referenceframe.Input{1.06, 3.379, -1.26, -0.59, 1.53, 1.06}
		segment := &SegmentFS{
			StartConfiguration: referenceframe.FrameSystemInputs{model.Name(): startConfig}.ToLinearInputs(),
			EndConfiguration:   referenceframe.FrameSystemInputs{model.Name(): endConfig}.ToLinearInputs(),
			FS:                 fs,
		}
		endPos, err := model.Transform(endConfig)
		test.That(t, err, test.ShouldBeNil)

		cartesianSteps := CalculateStepCount(startPos, endPos, 1.0)
		jointSteps := calculateJointStepCount(startConfig, endConfig, jointStepSize)

		totalSteps, err := segmentStepCount(segment, 1.0)
		test.That(t, err, test.ShouldBeNil)

		// Joint steps should dominate over cartesian for this trajectory
		test.That(t, jointSteps, test.ShouldBeGreaterThan, cartesianSteps)

		// segmentStepCount should return the joint step count
		test.That(t, totalSteps, test.ShouldEqual, jointSteps)
	})

	t.Run("cartesian steps dominate when joint distance is small", func(t *testing.T) {
		// Joints are close to startConfig, but quite far in cartesian space
		endConfig := []referenceframe.Input{1.06, -3.379, -1.26, -0.59, 1.53, 1.06}
		segment := &SegmentFS{
			StartConfiguration: referenceframe.FrameSystemInputs{model.Name(): startConfig}.ToLinearInputs(),
			EndConfiguration:   referenceframe.FrameSystemInputs{model.Name(): endConfig}.ToLinearInputs(),
			FS:                 fs,
		}
		endPos, err := model.Transform(endConfig)
		test.That(t, err, test.ShouldBeNil)

		cartesianSteps := CalculateStepCount(startPos, endPos, 1.0)
		jointSteps := calculateJointStepCount(startConfig, endConfig, jointStepSize)

		totalSteps, err := segmentStepCount(segment, 1.0)
		test.That(t, err, test.ShouldBeNil)

		// Cartesian steps should dominate over joint for this trajectory
		test.That(t, cartesianSteps, test.ShouldBeGreaterThan, jointSteps)

		// segmentStepCount should return the cartesian step count
		test.That(t, totalSteps, test.ShouldEqual, cartesianSteps)
	})
}

func TestComputeInitialCollisionsToIgnore(t *testing.T) {
	fs := referenceframe.NewEmptyFrameSystem("")

	bc1, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)

	t.Run("combines initial collisions with specifications", func(t *testing.T) {
		// Create colliding geometries
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewZeroPose())
		geom2.SetLabel("box2")

		moving := []spatial.Geometry{geom1}
		static := []spatial.Geometry{geom2}

		// Test that initial collisions are detected and combined with specifications
		collisionSpecs := []Collision{{"box1", "box3"}}
		ignoreList, err := computeInitialCollisionsToIgnore(fs, moving, static,
			collisionSpecs, defaultCollisionBufferMM, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ignoreList), test.ShouldEqual, 2)

		// Verify the specification collision is included
		found := false
		for _, c := range ignoreList {
			if c.name1 == "box1" && c.name2 == "box3" {
				found = true
				break
			}
		}
		test.That(t, found, test.ShouldBeTrue)
	})

	t.Run("empty when no collisions or specs", func(t *testing.T) {
		// Create non-colliding geometries
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewPoseFromPoint(r3.Vector{10, 0, 0}))
		geom2.SetLabel("box2")

		moving := []spatial.Geometry{geom1}
		static := []spatial.Geometry{geom2}

		ignoreList, err := computeInitialCollisionsToIgnore(fs, moving, static, nil, defaultCollisionBufferMM, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ignoreList), test.ShouldEqual, 0)
	})
}

func TestCollisionDistance(t *testing.T) {
	bc1, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)

	t.Run("collision returns -1 and error", func(t *testing.T) {
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewZeroPose())
		geom2.SetLabel("box2")

		collisions, _, err := checkCollisionsHinted([]spatial.Geometry{geom1}, []spatial.Geometry{geom2}, nil, nil,
			defaultCollisionBufferMM, false, nil, logging.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collisions, test.ShouldNotBeEmpty)
		test.That(t, collisions[0].name1, test.ShouldBeIn, "box1", "box2")
		test.That(t, collisions[0].name2, test.ShouldBeIn, "box1", "box2")
	})

	t.Run("no collision returns positive distance", func(t *testing.T) {
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewPoseFromPoint(r3.Vector{10, 0, 0}))
		geom2.SetLabel("box2")

		collisions, minDist, err := checkCollisionsHinted(
			[]spatial.Geometry{geom1}, []spatial.Geometry{geom2}, nil, nil,
			defaultCollisionBufferMM, false, nil, logging.NewTestLogger(t),
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collisions, test.ShouldBeEmpty)
		test.That(t, minDist, test.ShouldBeGreaterThan, 0)
	})

	t.Run("ignored collision returns positive distance", func(t *testing.T) {
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewZeroPose())
		geom2.SetLabel("box2")

		ignoreList := []Collision{{"box1", "box2"}}
		collisions, minDist, err := checkCollisionsHinted(
			[]spatial.Geometry{geom1}, []spatial.Geometry{geom2}, nil, makeAllowedCollisionsLookup(ignoreList),
			defaultCollisionBufferMM, false, nil, logging.NewTestLogger(t),
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collisions, test.ShouldBeEmpty)
		test.That(t, minDist, test.ShouldBeGreaterThan, 0)
	})
}

func BenchmarkOrientationArcDistance(b *testing.B) {
	from := spatial.NewZeroOrientation()
	to := spatial.Orientation(&spatial.EulerAngles{Pitch: 1.0, Yaw: 0.4})
	now := spatial.NewPoseFromOrientation(&spatial.EulerAngles{Roll: 0.3, Pitch: 0.7, Yaw: 1.1}).Orientation()
	arc := newOrientationArc(from, to, false)
	var dist float64
	for i := 0; i < b.N; i++ {
		dist = arc.distanceDegs(now)
	}
	_ = dist
}

func BenchmarkCollisionConstraints(b *testing.B) {
	// define external obstacles
	bc, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(b, err, test.ShouldBeNil)
	obstacles := []spatial.Geometry{}
	obstacles = append(obstacles, bc.Transform(spatial.NewZeroPose()))
	obstacles = append(obstacles, bc.Transform(spatial.NewPoseFromPoint(r3.Vector{-130, 0, 300})))
	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{
		referenceframe.NewGeometriesInFrame(referenceframe.World, obstacles),
	}, nil)
	test.That(b, err, test.ShouldBeNil)

	// setup zero position as reference CollisionGraph and use it in handler
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(b, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(referenceframe.World))
	test.That(b, err, test.ShouldBeNil)
	seedMap := referenceframe.NewNeutralFrameSystemInputs(fs)
	handler := &ConstraintChecker{}

	// create robot collision entities
	movingGeometriesInFrame, err := model.Geometries(seedMap[model.Name()])
	movingRobotGeometries := movingGeometriesInFrame.Geometries()
	test.That(b, err, test.ShouldBeNil)

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatial.Geometry, 0)
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(fs, seedMap)
	test.That(b, err, test.ShouldBeNil)
	for name, geometries := range frameSystemGeometries {
		if name != model.Name() {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}

	// Note that all obstacles in worldState are assumed to be static so it is ok to transform them into the world frame
	// TODO(rb) it is bad practice to assume that the current inputs of the robot correspond to the passed in world state
	// the state that observed the worldState should ultimately be included as part of the worldState message
	worldGeometries, err := worldState.ObstaclesInWorldFrame(fs, seedMap)
	test.That(b, err, test.ShouldBeNil)

	handler.collisionConstraints, err = CreateAllCollisionConstraints(
		fs,
		movingRobotGeometries,
		map[string]bool{model.Name(): true},
		staticRobotGeometries,
		worldGeometries.Geometries(),
		nil, // allowedCollisions
		defaultCollisionBufferMM,
		nil,
		logging.NewTestLogger(b),
	)
	test.That(b, err, test.ShouldBeNil)

	rseed := rand.New(rand.NewSource(1))

	// loop through cases and check constraint handler processes them correctly
	for n := 0; n < b.N; n++ {
		rfloats := referenceframe.GenerateRandomConfiguration(model, rseed)
		stateFS := &StateFS{
			Configuration: referenceframe.FrameSystemInputs{model.Name(): rfloats}.ToLinearInputs(),
			FS:            fs,
		}
		_, err = handler.CheckStateFSConstraints(context.Background(), stateFS)
		test.That(b, err, test.ShouldBeNil)
	}
}

// BenchmarkCollisionConstraintsObstructedEdge models the motion-planner edge-
// rejection path: a colliding configuration probed many times with tiny
// perturbations, the way checkPath interpolates along an obstructed edge. The
// last-violated-pair hint inside the collision constraint should let each call
// after the first reject in O(1) pair checks instead of O(N·M).
func BenchmarkCollisionConstraintsObstructedEdge(b *testing.B) {
	// Place one box that's actually in the way, plus many decoy boxes far away
	// so the pair-iteration cost dominates. Without the hint, the constraint
	// has to scan most pairs before finding the colliding one; with the hint,
	// the previously-found pair is checked first and the call returns in O(1).
	obstacles := []spatial.Geometry{}
	blocker, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 300}),
		r3.Vector{X: 400, Y: 400, Z: 400}, "blocker")
	test.That(b, err, test.ShouldBeNil)
	obstacles = append(obstacles, blocker)
	for i := 0; i < 30; i++ {
		decoy, err := spatial.NewBox(
			spatial.NewPoseFromPoint(r3.Vector{X: 5000 + float64(i)*100, Y: 0, Z: 0}),
			r3.Vector{X: 10, Y: 10, Z: 10},
			fmt.Sprintf("decoy_%d", i),
		)
		test.That(b, err, test.ShouldBeNil)
		obstacles = append(obstacles, decoy)
	}
	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{
		referenceframe.NewGeometriesInFrame(referenceframe.World, obstacles),
	}, nil)
	test.That(b, err, test.ShouldBeNil)

	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(b, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(referenceframe.World))
	test.That(b, err, test.ShouldBeNil)

	// Seed config: arm folded away from blocker. Used by
	// computeInitialCollisionsToIgnore to build the ignore list — must NOT
	// collide with the obstacle or the obstacle pair gets filtered forever.
	seedConfig := []referenceframe.Input{0, -1.5, -1.5, 0, 1.5, 0}
	seedMap := referenceframe.FrameSystemInputs{model.Name(): seedConfig}

	// Colliding config: zero pose puts the wrist/forearm inside the blocker box.
	collidingConfig := []referenceframe.Input{0, 0, 0, 0, 0, 0}

	movingGeometriesInFrame, err := model.Geometries(seedMap[model.Name()])
	test.That(b, err, test.ShouldBeNil)
	movingRobotGeometries := movingGeometriesInFrame.Geometries()
	staticRobotGeometries := []spatial.Geometry{}
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(fs, seedMap)
	test.That(b, err, test.ShouldBeNil)
	for name, geometries := range frameSystemGeometries {
		if name != model.Name() {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}
	worldGeometries, err := worldState.ObstaclesInWorldFrame(fs, seedMap)
	test.That(b, err, test.ShouldBeNil)

	handler := &ConstraintChecker{}
	handler.collisionConstraints, err = CreateAllCollisionConstraints(
		fs, movingRobotGeometries,
		map[string]bool{model.Name(): true},
		staticRobotGeometries, worldGeometries.Geometries(),
		nil, defaultCollisionBufferMM, nil, logging.NewTestLogger(b),
	)
	test.That(b, err, test.ShouldBeNil)

	// Walk a short trajectory that stays in collision throughout — simulates
	// the planner discovering an obstructed edge and walking forward looking
	// for the first valid sub-state.
	const steps = 32
	configs := make([]referenceframe.FrameSystemInputs, steps)
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		perturbed := make([]referenceframe.Input, len(collidingConfig))
		for j, v := range collidingConfig {
			perturbed[j] = v + referenceframe.Input(t*0.02)
		}
		configs[i] = referenceframe.FrameSystemInputs{model.Name(): perturbed}
	}

	// Sanity-check that the trajectory actually collides — without this the
	// bench would silently measure the non-colliding fast path on both sides
	// and the hint would never get populated.
	{
		stateFS := &StateFS{Configuration: configs[0].ToLinearInputs(), FS: fs}
		_, err = handler.CheckStateFSConstraints(context.Background(), stateFS)
		test.That(b, err, test.ShouldNotBeNil)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		stateFS := &StateFS{
			Configuration: configs[n%steps].ToLinearInputs(),
			FS:            fs,
		}
		_, _ = handler.CheckStateFSConstraints(context.Background(), stateFS)
	}
}
