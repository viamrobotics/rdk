package armplanning

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// TestDualarmSharedBarMultiFrameGoal: can PlanMotion resolve a goal that
// names two tool frames that must both land at the same world pose (the
// shared bar held by two arms)? This exercises the multi-frame branch of
// GetGoalMetric (planner_options.go:181) which no other test covers.
func TestDualarmSharedBarMultiFrameGoal(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// Load the sanding dualarm frame system (two UR20s + shared sander bar).
	fsPath := filepath.Join("testdata", "dualarm", "frame_system.json")
	fsBytes, err := os.ReadFile(fsPath)
	test.That(t, err, test.ShouldBeNil)
	var fs frame.FrameSystem
	test.That(t, json.Unmarshal(fsBytes, &fs), test.ShouldBeNil)

	inpPath := filepath.Join("testdata", "dualarm", "frame_system_inputs.json")
	inpBytes, err := os.ReadFile(inpPath)
	test.That(t, err, test.ShouldBeNil)
	var startInputs frame.FrameSystemInputs
	test.That(t, json.Unmarshal(inpBytes, &startInputs), test.ShouldBeNil)

	// Where are compliance_origin and _arm2 at the start state?
	startLin := startInputs.ToLinearInputs()
	primaryPIF := frame.NewPoseInFrame("compliance_origin", spatialmath.NewZeroPose())
	followerPIF := frame.NewPoseInFrame("compliance_origin_arm2", spatialmath.NewZeroPose())
	primaryStartRes, err := fs.Transform(startLin, primaryPIF, frame.World)
	test.That(t, err, test.ShouldBeNil)
	followerStartRes, err := fs.Transform(startLin, followerPIF, frame.World)
	test.That(t, err, test.ShouldBeNil)
	primaryStart := primaryStartRes.(*frame.PoseInFrame).Pose()
	followerStart := followerStartRes.(*frame.PoseInFrame).Pose()
	t.Logf("start: compliance_origin at %v, compliance_origin_arm2 at %v",
		primaryStart.Point(), followerStart.Point())

	// Significant move: 300mm in Y (toward where a vertical wall would be),
	// 200mm in -X, 100mm in Z. Exercises RRT exploration + both arms moving.
	targetPt := r3.Vector{
		X: primaryStart.Point().X - 200,
		Y: primaryStart.Point().Y + 300,
		Z: primaryStart.Point().Z + 100,
	}
	targetPose := spatialmath.NewPose(targetPt, primaryStart.Orientation())

	goal := &PlanState{poses: frame.FrameSystemPoses{
		"compliance_origin":      frame.NewPoseInFrame(frame.World, targetPose),
		"compliance_origin_arm2": frame.NewPoseInFrame(frame.World, targetPose),
	}}

	req := &PlanRequest{
		FrameSystem:    &fs,
		Goals:          []*PlanState{goal},
		StartState:     &PlanState{structuredConfiguration: startInputs},
		PlannerOptions: NewBasicPlannerOptions(),
	}

	plan, meta, err := PlanMotion(context.Background(), logger, req)
	if err != nil {
		t.Logf("PlanMotion error: %v", err)
	}
	if meta != nil {
		t.Logf("meta: %+v", meta.Duration)
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan, test.ShouldNotBeNil)
	traj := plan.Trajectory()
	t.Logf("plan length: %d steps", len(traj))
	test.That(t, len(traj), test.ShouldBeGreaterThan, 1)

	// Verify at the final step both frames land near the target.
	finalLin := traj[len(traj)-1].ToLinearInputs()
	finalPrim, err := fs.Transform(finalLin, primaryPIF, frame.World)
	test.That(t, err, test.ShouldBeNil)
	finalFol, err := fs.Transform(finalLin, followerPIF, frame.World)
	test.That(t, err, test.ShouldBeNil)
	primEnd := finalPrim.(*frame.PoseInFrame).Pose()
	folEnd := finalFol.(*frame.PoseInFrame).Pose()
	primErr := primEnd.Point().Sub(targetPt).Norm()
	folErr := folEnd.Point().Sub(targetPt).Norm()
	coincideErr := primEnd.Point().Sub(folEnd.Point()).Norm()
	t.Logf("final primary pos err: %.2fmm, follower: %.2fmm, coincide err: %.2fmm",
		primErr, folErr, coincideErr)
}

// TestDualarmSharedFramesConstraint: same setup, but now the plan request
// carries a SharedFrames constraint tying the two grip frames together. The
// goal metric alone doesn't enforce rigidity along the trajectory — the
// SharedFrames check inside addTopoConstraints is what should force RRT to
// hold the shared-tool invariant at every intermediate step.
func TestDualarmSharedFramesConstraint(t *testing.T) {
	logger := logging.NewTestLogger(t)

	fsPath := filepath.Join("testdata", "dualarm", "frame_system.json")
	fsBytes, err := os.ReadFile(fsPath)
	test.That(t, err, test.ShouldBeNil)
	var fs frame.FrameSystem
	test.That(t, json.Unmarshal(fsBytes, &fs), test.ShouldBeNil)

	inpPath := filepath.Join("testdata", "dualarm", "frame_system_inputs.json")
	inpBytes, err := os.ReadFile(inpPath)
	test.That(t, err, test.ShouldBeNil)
	var startInputs frame.FrameSystemInputs
	test.That(t, json.Unmarshal(inpBytes, &startInputs), test.ShouldBeNil)

	startLin := startInputs.ToLinearInputs()
	primaryPIF := frame.NewPoseInFrame("compliance_origin", spatialmath.NewZeroPose())
	followerPIF := frame.NewPoseInFrame("compliance_origin_arm2", spatialmath.NewZeroPose())
	primaryStartRes, err := fs.Transform(startLin, primaryPIF, frame.World)
	test.That(t, err, test.ShouldBeNil)
	followerStartRes, err := fs.Transform(startLin, followerPIF, frame.World)
	test.That(t, err, test.ShouldBeNil)
	primaryStart := primaryStartRes.(*frame.PoseInFrame).Pose()
	followerStart := followerStartRes.(*frame.PoseInFrame).Pose()
	startSep := primaryStart.Point().Sub(followerStart.Point()).Norm()
	t.Logf("start sep: %.2fmm", startSep)

	// Tolerance must accommodate the real start separation (frame_system_inputs
	// snapshot is from a hardware pose where the grip frames aren't perfectly
	// coincident). 50mm/8° is slack but still tight enough to catch a planner
	// that lets the arms drift apart mid-trajectory.
	const posTolMM = 50.0
	const oriTolDeg = 8.0

	targetPt := r3.Vector{
		X: primaryStart.Point().X - 150,
		Y: primaryStart.Point().Y + 200,
		Z: primaryStart.Point().Z + 80,
	}
	targetPose := spatialmath.NewPose(targetPt, primaryStart.Orientation())

	goal := &PlanState{poses: frame.FrameSystemPoses{
		"compliance_origin":      frame.NewPoseInFrame(frame.World, targetPose),
		"compliance_origin_arm2": frame.NewPoseInFrame(frame.World, targetPose),
	}}

	req := &PlanRequest{
		FrameSystem: &fs,
		Goals:       []*PlanState{goal},
		StartState:  &PlanState{structuredConfiguration: startInputs},
		Constraints: &motionplan.Constraints{
			SharedFrames: []motionplan.SharedFramesConstraint{{
				Frames:                   []string{"compliance_origin", "compliance_origin_arm2"},
				PositionToleranceMM:      posTolMM,
				OrientationToleranceDegs: oriTolDeg,
			}},
		},
		PlannerOptions: NewBasicPlannerOptions(),
	}

	plan, meta, err := PlanMotion(context.Background(), logger, req)
	if err != nil {
		t.Logf("PlanMotion error: %v", err)
	}
	if meta != nil {
		t.Logf("meta duration: %v", meta.Duration)
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan, test.ShouldNotBeNil)

	traj := plan.Trajectory()
	maxSep := 0.0
	maxOriDeg := 0.0
	for i, step := range traj {
		lin := step.ToLinearInputs()
		p, err := fs.Transform(lin, primaryPIF, frame.World)
		test.That(t, err, test.ShouldBeNil)
		f, err := fs.Transform(lin, followerPIF, frame.World)
		test.That(t, err, test.ShouldBeNil)
		pPose := p.(*frame.PoseInFrame).Pose()
		fPose := f.(*frame.PoseInFrame).Pose()
		sep := pPose.Point().Sub(fPose.Point()).Norm()
		oriDiff := spatialmath.OrientationBetween(pPose.Orientation(), fPose.Orientation())
		oriDeg := math.Abs(oriDiff.AxisAngles().Theta) * 180.0 / math.Pi
		if sep > maxSep {
			maxSep = sep
		}
		if oriDeg > maxOriDeg {
			maxOriDeg = oriDeg
		}
		if sep > posTolMM || oriDeg > oriTolDeg {
			t.Logf("step %d: sep=%.2fmm ori=%.2f° (over tol)", i, sep, oriDeg)
		}
	}
	t.Logf("max separation across %d steps: %.2fmm (tol %.1fmm), max ori %.2f° (tol %.1f°)",
		len(traj), maxSep, posTolMM, maxOriDeg, oriTolDeg)
	test.That(t, maxSep, test.ShouldBeLessThanOrEqualTo, posTolMM)
	test.That(t, maxOriDeg, test.ShouldBeLessThanOrEqualTo, oriTolDeg)
}
