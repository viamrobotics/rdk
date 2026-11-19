package motionplan

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// This edge was returned by native planning for an xArm6 moving around a box.
// Both endpoints are clear, but the wrist intersects the box at fraction 0.42.
// Keep the joint values fixed so reproduction does not depend on search seeds,
// worker scheduling, or a particular choice of sampling policy.
func TestCollisionSamplingRecordedWristCrossing(t *testing.T) {
	ctx := context.Background()
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("recorded-wrist-crossing")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)
	from := referenceframe.FrameSystemInputs{"arm": {
		0.13351501257437082, 0.3658842177745062, -0.9031143910761043,
		0.22067163801214268, 0.5492212326125323, -0.07468806385086137,
	}}.ToLinearInputs()
	to := referenceframe.FrameSystemInputs{"arm": {
		-9.004630000282042e-08, 0.5841754423045156, -1.0737874199246076,
		-1.7935669640847313e-06, 0.48961297536761605, 6.839978786341548e-07,
	}}.ToLinearInputs()
	seed := referenceframe.FrameSystemInputs{"arm": {0, 0, 0, 0, 0, 0}}.ToLinearInputs()
	blocker, err := spatialmath.NewBox(
		spatialmath.NewPose(r3.Vector{X: 357, Z: 112}, &spatialmath.R4AA{Theta: math.Pi, RY: 1}),
		r3.Vector{X: 20, Y: 2000, Z: 60}, "blocker",
	)
	test.That(t, err, test.ShouldBeNil)
	witness, err := referenceframe.InterpolateFS(fs, from, to, 0.42)
	test.That(t, err, test.ShouldBeNil)
	geometries, err := model.Geometries(witness.Get("arm"))
	test.That(t, err, test.ShouldBeNil)
	var wrist spatialmath.Geometry
	for _, geometry := range geometries.Geometries() {
		if geometry.Label() == "wrist_link" || geometry.Label() == "arm:wrist_link" {
			wrist = geometry
		}
	}
	test.That(t, wrist, test.ShouldNotBeNil)
	collides, _, err := wrist.CollidesWith(blocker, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	distance, err := wrist.DistanceFrom(blocker)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, distance, test.ShouldAlmostEqual, -0.29193983, 1e-5)
	t.Logf("witness fraction=0.42, zero-buffer signed wrist/box distance=%.9f mm", distance)

	for _, tc := range []struct {
		name       string
		bufferMM   float64
		resolution float64
	}{
		{"original_settings", 1e-8, 2},
		{"larger_buffer_control", 1, 2},
		{"finer_resolution_control", 1e-8, 0.1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checker := samplingCollisionChecker(t, fs, seed, []spatialmath.Geometry{blocker}, tc.bufferMM)
			checkSamplingCrossing(t, ctx, checker, &SegmentFS{
				FS: fs, StartConfiguration: from, EndConfiguration: to,
			}, witness, tc.resolution)
		})
	}
}

// Two independently rotating links carry 1 mm-radius spheres 1000 mm from
// their axes. The spheres exchange positions and coincide halfway through the
// motion. Frame-origin and joint increments alone do not bound their clearance.
func TestCollisionSamplingRelativeMotion(t *testing.T) {
	fs := referenceframe.NewEmptyFrameSystem("relative-motion-crossing")
	for _, name := range []string{"left", "right"} {
		joint, err := referenceframe.NewRotationalFrame(name, spatialmath.R4AA{RZ: 1}, referenceframe.Limit{Min: -2, Max: 2})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fs.AddFrame(joint, fs.World()), test.ShouldBeNil)
		sphere, err := spatialmath.NewSphere(spatialmath.NewPoseFromPoint(r3.Vector{X: 1000}), 1, name+"-sphere")
		test.That(t, err, test.ShouldBeNil)
		link, err := referenceframe.NewStaticFrameWithGeometry(name+"-link", spatialmath.NewZeroPose(), sphere)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fs.AddFrame(link, joint), test.ShouldBeNil)
	}
	from := referenceframe.FrameSystemInputs{"left": {0.7}, "right": {1.3}}.ToLinearInputs()
	to := referenceframe.FrameSystemInputs{"left": {1.3}, "right": {0.7}}.ToLinearInputs()
	witness, err := referenceframe.InterpolateFS(fs, from, to, 0.5)
	test.That(t, err, test.ShouldBeNil)
	geometries, err := referenceframe.FrameSystemGeometriesLinearInputs(fs, witness)
	test.That(t, err, test.ShouldBeNil)
	left := geometries["left-link"].Geometries()[0]
	right := geometries["right-link"].Geometries()[0]
	distance, err := left.DistanceFrom(right)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, distance, test.ShouldAlmostEqual, -2, 1e-6)
	collides, _, err := left.CollidesWith(right, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, collides, test.ShouldBeTrue)
	t.Logf("witness fraction=0.5, zero-buffer signed sphere/sphere distance=%.9f mm", distance)
	checker := samplingCollisionChecker(t, fs, from, nil, 1e-8)
	checkSamplingCrossing(t, context.Background(), checker, &SegmentFS{
		FS: fs, StartConfiguration: from, EndConfiguration: to,
	}, witness, 1)
}

func samplingCollisionChecker(t *testing.T, fs *referenceframe.FrameSystem, seed *referenceframe.LinearInputs,
	obstacles []spatialmath.Geometry, bufferMM float64,
) *ConstraintChecker {
	t.Helper()
	geometries, err := referenceframe.FrameSystemGeometriesLinearInputs(fs, seed)
	test.That(t, err, test.ShouldBeNil)
	var robot []spatialmath.Geometry
	moving := map[string]bool{}
	for name, group := range geometries {
		moving[name] = true
		robot = append(robot, group.Geometries()...)
	}
	checker, err := NewConstraintChecker(bufferMM, nil, nil, nil, fs, robot, nil, moving, seed,
		referenceframe.NewGeometriesInFrame(referenceframe.World, obstacles), logging.NewTestLogger(t), nil)
	test.That(t, err, test.ShouldBeNil)
	return checker
}

func checkSamplingCrossing(t *testing.T, ctx context.Context, checker *ConstraintChecker,
	segment *SegmentFS, witness *referenceframe.LinearInputs, resolution float64,
) {
	t.Helper()
	for _, endpoint := range []*referenceframe.LinearInputs{segment.StartConfiguration, segment.EndConfiguration} {
		_, err := checker.CheckStateFSConstraints(ctx, &StateFS{FS: segment.FS, Configuration: endpoint})
		test.That(t, err, test.ShouldBeNil)
	}
	_, err := checker.CheckStateFSConstraints(ctx, &StateFS{FS: segment.FS, Configuration: witness})
	test.That(t, err, test.ShouldNotBeNil)

	// Diagnose the current grid without requiring a particular number of samples
	// or requiring a future fix to use uniform sampling at all.
	samples, err := InterpolateSegmentFS(segment, resolution)
	test.That(t, err, test.ShouldBeNil)
	invalid := 0
	for _, sample := range samples {
		if _, err := checker.CheckStateFSConstraints(ctx, &StateFS{FS: segment.FS, Configuration: sample}); err != nil {
			invalid++
		}
	}
	t.Logf("buffer=%.9g mm, resolution=%g, grid states=%d, colliding grid states=%d",
		checker.collisionBufferMM, resolution, len(samples), invalid)
	_, err = checker.CheckStateConstraintsAcrossSegmentFS(ctx, segment, resolution, true)
	if err == nil {
		t.Fatal("segment accepted despite a colliding intermediate configuration")
	}
}
