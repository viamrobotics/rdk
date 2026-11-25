package armplanning

import (
	"context"
	"maps"
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

func frozenTestStart() frame.FrameSystemInputs {
	return frame.FrameSystemInputs{
		"gantryX": {300},
		"gantryY": {-50},
		"xArm6":   {0.1, 0.2, -0.3, 0.4, 0.5, 0.6},
		"UR5e":    {0.3, -0.5, 0.4, 0.1, 0.2, -0.1},
	}
}

func withoutFrames(in frame.FrameSystemInputs, names ...string) frame.FrameSystemInputs {
	out := maps.Clone(in)
	for _, n := range names {
		delete(out, n)
	}
	return out
}

func geometriesByLabel(t *testing.T, fs *frame.FrameSystem, inputs frame.FrameSystemInputs) map[string]spatialmath.Geometry {
	t.Helper()
	gifs, err := frame.FrameSystemGeometries(fs, inputs)
	test.That(t, err, test.ShouldBeNil)
	out := map[string]spatialmath.Geometry{}
	for _, gif := range gifs {
		for _, g := range gif.Geometries() {
			out[g.Label()] = g
		}
	}
	return out
}

func TestFreezeFramesPreservesGeometry(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenTestStart()
	frozenFS, _, err := freezeFrames(fs, []string{"gantryX", "xArm6"}, start)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(frozenFS.Frame("gantryX").DoF()), test.ShouldEqual, 0)
	test.That(t, len(frozenFS.Frame("xArm6").DoF()), test.ShouldEqual, 0)

	before := geometriesByLabel(t, fs, start)
	after := geometriesByLabel(t, frozenFS, withoutFrames(start, "gantryX", "xArm6"))
	test.That(t, len(before), test.ShouldBeGreaterThan, 0)
	for label, g := range before {
		frozen, ok := after[label]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, spatialmath.PoseAlmostEqual(g.Pose(), frozen.Pose()), test.ShouldBeTrue)
	}
}

func TestFreezeFramesKeepsChildPoses(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenTestStart()
	frozenFS, _, err := freezeFrames(fs, []string{"UR5e"}, start)
	test.That(t, err, test.ShouldBeNil)

	cameraPose := func(fs *frame.FrameSystem, in frame.FrameSystemInputs) spatialmath.Pose {
		tf, err := fs.Transform(in.ToLinearInputs(), frame.NewZeroPoseInFrame("urCamera"), frame.World)
		test.That(t, err, test.ShouldBeNil)
		return tf.(*frame.PoseInFrame).Pose()
	}
	test.That(t, spatialmath.PoseAlmostEqual(
		cameraPose(fs, start),
		cameraPose(frozenFS, withoutFrames(start, "UR5e")),
	), test.ShouldBeTrue)
}

func TestFreezeFramesReturnsInputs(t *testing.T) {
	start := frozenTestStart()
	_, frozen, err := freezeFrames(makeTestFS(t), []string{"gantryX", "xArm6"}, start)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frozen, test.ShouldResemble, frame.FrameSystemInputs{
		"gantryX": start["gantryX"],
		"xArm6":   start["xArm6"],
	})
}

func TestFreezeFramesErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames []string
		start  frame.FrameSystemInputs
		errHas string
	}{
		{"unknown frame", []string{"nope"}, frozenTestStart(), "nope"},
		{"zero DoF frame", []string{"urOffset"}, frozenTestStart(), "urOffset"},
		{"missing from start", []string{"gantryY"}, withoutFrames(frozenTestStart(), "gantryY"), "gantryY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := freezeFrames(makeTestFS(t), tc.frames, tc.start)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errHas)
		})
	}
}

func TestFreezeFramesDoesNotMutateInput(t *testing.T) {
	fs := makeTestFS(t)
	_, _, err := freezeFrames(fs, []string{"gantryX"}, frozenTestStart())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(fs.Frame("gantryX").DoF()), test.ShouldEqual, 1)
}

func frozenPlanStart(fs *frame.FrameSystem) frame.FrameSystemInputs {
	start := frame.NewNeutralFrameSystemInputs(fs)
	start["gantryX"] = []frame.Input{100}
	start["gantryY"] = []frame.Input{-50}
	start["xArm6"] = []frame.Input{0, -0.3, -0.5, 0, 0.8, 0}
	return start
}

func framePoseInWorld(t *testing.T, fs *frame.FrameSystem, in frame.FrameSystemInputs, name string) spatialmath.Pose {
	t.Helper()
	tf, err := fs.Transform(in.ToLinearInputs(), frame.NewZeroPoseInFrame(name), frame.World)
	test.That(t, err, test.ShouldBeNil)
	return tf.(*frame.PoseInFrame).Pose()
}

func frozenPlanRequest(
	t *testing.T, fs *frame.FrameSystem, start frame.FrameSystemInputs, frozen []string, goals ...*PlanState,
) *PlanRequest {
	t.Helper()
	opts := NewBasicPlannerOptions()
	opts.RandomSeed = 1
	constraints := motionplan.NewEmptyConstraints()
	for _, name := range frozen {
		constraints.AddFrozenFrameConstraint(motionplan.FrozenFrameConstraint{Frame: name})
	}
	return &PlanRequest{
		FrameSystem:    fs,
		StartState:     NewPlanState(nil, start),
		Goals:          goals,
		Constraints:    constraints,
		PlannerOptions: opts,
	}
}

func xArmOffsetGoal(t *testing.T, fs *frame.FrameSystem, start frame.FrameSystemInputs, offset r3.Vector) *PlanState {
	t.Helper()
	p := framePoseInWorld(t, fs, start, "xArm6")
	goal := spatialmath.NewPose(p.Point().Add(offset), p.Orientation())
	return NewPlanState(frame.FrameSystemPoses{"xArm6": frame.NewPoseInFrame(frame.World, goal)}, nil)
}

func assertInputsConstant(t *testing.T, plan motionplan.Plan, name string, want []frame.Input) {
	t.Helper()
	traj := plan.Trajectory()
	test.That(t, len(traj), test.ShouldBeGreaterThan, 0)
	for _, step := range traj {
		test.That(t, step[name], test.ShouldResemble, want)
	}
}

func TestPlanMotionFrozenGantry(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	goal := xArmOffsetGoal(t, fs, start, r3.Vector{X: 100})
	plan, _, err := PlanMotion(context.Background(), logging.NewTestLogger(t),
		frozenPlanRequest(t, fs, start, []string{"gantryX", "gantryY"}, goal))
	test.That(t, err, test.ShouldBeNil)
	assertInputsConstant(t, plan, "gantryX", start["gantryX"])
	assertInputsConstant(t, plan, "gantryY", start["gantryY"])

	traj := plan.Trajectory()
	end := framePoseInWorld(t, fs, traj[len(traj)-1], "xArm6")
	test.That(t, end.Point().Distance(goal.Poses()["xArm6"].Pose().Point()), test.ShouldBeLessThan, 1.0)
}

func TestPlanMotionFrozenArm(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	p := framePoseInWorld(t, fs, start, "UR5e")
	goal := NewPlanState(frame.FrameSystemPoses{"UR5e": frame.NewPoseInFrame(frame.World,
		spatialmath.NewPose(p.Point().Add(r3.Vector{Z: -100}), p.Orientation()))}, nil)
	plan, _, err := PlanMotion(context.Background(), logging.NewTestLogger(t),
		frozenPlanRequest(t, fs, start, []string{"xArm6"}, goal))
	test.That(t, err, test.ShouldBeNil)
	assertInputsConstant(t, plan, "xArm6", start["xArm6"])
}

func TestPlanMotionFrozenGeometryIsObstacle(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	forearm, ok := geometriesByLabel(t, fs, start)["UR5e:forearm_link"]
	test.That(t, ok, test.ShouldBeTrue)
	p := framePoseInWorld(t, fs, start, "xArm6")
	goal := NewPlanState(frame.FrameSystemPoses{"xArm6": frame.NewPoseInFrame(frame.World,
		spatialmath.NewPose(forearm.Pose().Point(), p.Orientation()))}, nil)

	_, _, err := PlanMotion(context.Background(), logging.NewTestLogger(t), frozenPlanRequest(t, fs, start, nil, goal))
	test.That(t, err, test.ShouldBeNil)

	_, _, err = PlanMotion(context.Background(), logging.NewTestLogger(t),
		frozenPlanRequest(t, fs, start, []string{"UR5e"}, goal))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "UR5e:forearm_link")
}

func TestPlanMotionFrozenGoalConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		gantryX []frame.Input
		wantErr bool
	}{
		{"matches start", []frame.Input{100}, false},
		{"differs from start", []frame.Input{200}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := makeTestFS(t)
			start := frozenPlanStart(fs)
			goalConf := maps.Clone(start)
			goalConf["xArm6"] = []frame.Input{0.2, -0.3, -0.5, 0, 0.8, 0}
			goalConf["gantryX"] = tc.gantryX
			_, _, err := PlanMotion(context.Background(), logging.NewTestLogger(t),
				frozenPlanRequest(t, fs, start, []string{"gantryX"}, NewPlanState(nil, goalConf)))
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "frozen")
				test.That(t, err.Error(), test.ShouldContainSubstring, "gantryX")
			} else {
				test.That(t, err, test.ShouldBeNil)
			}
		})
	}
}

func TestPlanMotionFrozenImmovableGoal(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	goal := xArmOffsetGoal(t, fs, start, r3.Vector{X: 100})
	_, _, err := PlanMotion(context.Background(), logging.NewTestLogger(t),
		frozenPlanRequest(t, fs, start, []string{"xArm6", "gantryX", "gantryY"}, goal))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `no DoF moves "xArm6"`)
}

func TestPlanMotionFrozenDoesNotMutateRequest(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	goalConf := maps.Clone(start)
	goalConf["xArm6"] = []frame.Input{0.2, -0.3, -0.5, 0, 0.8, 0}
	goal := NewPlanState(nil, goalConf)
	req := frozenPlanRequest(t, fs, start, []string{"gantryX"}, goal)
	_, _, err := PlanMotion(context.Background(), logging.NewTestLogger(t), req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, req.FrameSystem, test.ShouldEqual, fs)
	test.That(t, len(fs.Frame("gantryX").DoF()), test.ShouldEqual, 1)
	test.That(t, req.StartState.Configuration()["gantryX"], test.ShouldResemble, []frame.Input{100})
	test.That(t, req.Goals[0], test.ShouldEqual, goal)
	test.That(t, goal.Configuration()["gantryX"], test.ShouldResemble, []frame.Input{100})
}

func TestPlanMotionFrozenPartialPlanCarriesInputs(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	reachable := xArmOffsetGoal(t, fs, start, r3.Vector{X: 100})
	unreachable := xArmOffsetGoal(t, fs, start, r3.Vector{Y: 21000})
	req := frozenPlanRequest(t, fs, start, []string{"gantryX", "gantryY"}, reachable, unreachable)
	req.PlannerOptions.ReturnPartialPlan = true
	plan, meta, err := PlanMotion(context.Background(), logging.NewTestLogger(t), req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, meta.Partial, test.ShouldBeTrue)
	assertInputsConstant(t, plan, "gantryX", start["gantryX"])
}

func TestFrozenFrameConstraintJSON(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	req := frozenPlanRequest(t, fs, start, []string{"gantryX"}, xArmOffsetGoal(t, fs, start, r3.Vector{X: 100}))
	path := filepath.Join(t.TempDir(), "req.json")
	test.That(t, req.WriteToFile(path), test.ShouldBeNil)
	got, err := ReadRequestFromFile(path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Constraints.FrozenFrameConstraint, test.ShouldResemble,
		[]motionplan.FrozenFrameConstraint{{Frame: "gantryX"}})
	raw, err := os.ReadFile(path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(raw), test.ShouldContainSubstring, `"frozen_frame_constraints"`)
}

func TestFrozenRequestCarriesRejectedSolutions(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	req := frozenPlanRequest(t, fs, start, []string{"gantryX"}, xArmOffsetGoal(t, fs, start, r3.Vector{X: 100}))
	rejected := maps.Clone(start)
	req.rejectedSolutions = map[*PlanState][]frame.FrameSystemInputs{req.StartState: {rejected}}

	planReq, _, err := frozenRequest(req)
	test.That(t, err, test.ShouldBeNil)
	got := planReq.rejectedSolutions[planReq.StartState]
	test.That(t, got, test.ShouldResemble, []frame.FrameSystemInputs{withoutFrames(rejected, "gantryX")})
	test.That(t, req.rejectedSolutions[req.StartState], test.ShouldResemble, []frame.FrameSystemInputs{rejected})
}

// The roadmap caches world-space swept volumes per chain, and a frozen frame's pose lives only in a
// static transform the roadmap keys never see, so roadmap results from one frozen pose would be
// reused at another without collision checks.
func TestFrozenFramesSkipRoadmap(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	goal := xArmOffsetGoal(t, fs, start, r3.Vector{X: 100})
	planReq, _, err := frozenRequest(frozenPlanRequest(t, fs, start, []string{"gantryX", "gantryY"}, goal))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, planReq.validatePlanRequest(), test.ShouldBeNil)

	ctx := context.Background()
	pc, err := NewPlanContext(ctx, logging.NewTestLogger(t), planReq, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)
	psc, err := NewPlanSegmentContext(ctx, pc, planReq.StartState.LinearConfiguration(), goal.Poses())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, getRoadmap(psc, logging.NewTestLogger(t)), test.ShouldBeNil)
}

func TestPlanMotionFrozenAllowByFrameName(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenPlanStart(fs)
	req := frozenPlanRequest(t, fs, start, []string{"UR5e"}, xArmOffsetGoal(t, fs, start, r3.Vector{X: 100}))
	req.Constraints.CollisionSpecification = []motionplan.CollisionSpecification{{
		Allows: []motionplan.CollisionSpecificationAllowedFrameCollisions{{Frame1: "UR5e", Frame2: "xArm6"}},
	}}
	_, _, err := PlanMotion(context.Background(), logging.NewTestLogger(t), req)
	test.That(t, err, test.ShouldBeNil)
}
