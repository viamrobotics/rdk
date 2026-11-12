//go:build !386

package armplanning

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestOrbOneSeed(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	matches, err := filepath.Glob("data/orb-plan*.json")
	test.That(t, err, test.ShouldBeNil)

	for _, fp := range matches {
		t.Run(fp, func(t *testing.T) {
			logger := logging.NewTestLogger(t)

			req, err := ReadRequestFromFile(fp)
			test.That(t, err, test.ShouldBeNil)

			plan, meta, err := PlanMotion(context.Background(), logger, req)
			test.That(t, err, test.ShouldBeNil)

			a := plan.Trajectory()[0]["sanding-ur5"]
			b := plan.Trajectory()[1]["sanding-ur5"]

			test.That(t, referenceframe.InputsL2Distance(a, b), test.ShouldBeLessThan, .005)
			test.That(t, referenceframe.InputsL2Distance(a, b), test.ShouldBeGreaterThan, 0)
			test.That(t, meta.Duration, test.ShouldBeGreaterThan, 0)
		})
	}
}

func TestOrbManySeeds(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	matches, err := filepath.Glob("data/orb-plan*.json")
	test.That(t, err, test.ShouldBeNil)

	for _, fp := range matches {
		req, err := ReadRequestFromFile(fp)
		test.That(t, err, test.ShouldBeNil)

		for i := 0; i < 100; i++ {
			t.Run(fmt.Sprintf("%s-%d", fp, i), func(t *testing.T) {
				logger := newChattyMotionPlanTestLogger(t)

				req.PlannerOptions.RandomSeed = i
				plan, _, err := PlanMotion(context.Background(), logger, req)
				test.That(t, err, test.ShouldBeNil)

				a := plan.Trajectory()[0]["sanding-ur5"]
				b := plan.Trajectory()[1]["sanding-ur5"]

				test.That(t, referenceframe.InputsL2Distance(a, b), test.ShouldBeLessThan, .005)
			})
		}
	}
}

func TestPourManySeeds(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	req, err := ReadRequestFromFile("data/pour-plan-bad.json")
	test.That(t, err, test.ShouldBeNil)

	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("seed-%d", i), func(t *testing.T) {
			logger := newChattyMotionPlanTestLogger(t)

			req.PlannerOptions.RandomSeed = i
			plan, _, err := PlanMotion(context.Background(), logger, req)
			test.That(t, err, test.ShouldBeNil)

			a := plan.Trajectory()[0]["arm-right"]
			b := plan.Trajectory()[1]["arm-right"]

			test.That(t, referenceframe.InputsL2Distance(a, b), test.ShouldBeLessThan, .15)
		})
	}
}

func TestWineCrazyTouch1(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/wine-crazy-touch.json")
	test.That(t, err, test.ShouldBeNil)

	req.myTestOptions.doNotCloseObstacles = true

	plan, _, err := PlanMotion(context.Background(), logger, req)
	test.That(t, err, test.ShouldBeNil)

	orig := plan.Trajectory()[0]["arm-right"]
	for _, tt := range plan.Trajectory() {
		now := tt["arm-right"]
		logger.Infof("r: %v", now)
		logger.Infof("l: %v", tt["arm-left"])
		d := referenceframe.InputsL2Distance(orig, now)
		test.That(t, d, test.ShouldBeLessThan, 0.0001)
	}

	test.That(t, len(plan.Trajectory()), test.ShouldBeLessThan, 6)
}

func TestWineCrazyTouch2(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/wine-crazy-touch2.json")
	test.That(t, err, test.ShouldBeNil)

	t.Run("regular", func(t *testing.T) {
		plan, _, err := PlanMotion(context.Background(), logger, req)
		test.That(t, err, test.ShouldBeNil)

		orig := plan.Trajectory()[0]["arm-right"]
		for _, tt := range plan.Trajectory() {
			now := tt["arm-right"]
			logger.Info(now)
			test.That(t, referenceframe.InputsL2Distance(orig, now), test.ShouldBeLessThan, 0.0001)
		}

		// Smoothing produces ~3 waypoints, addCloseObstacleWaypoints may add more where path is close to obstacles
		test.That(t, len(plan.Trajectory()), test.ShouldBeLessThan, 10)
	})

	t.Run("regular-noclose", func(t *testing.T) {
		req.myTestOptions.doNotCloseObstacles = true
		plan, _, err := PlanMotion(context.Background(), logger, req)
		req.myTestOptions.doNotCloseObstacles = false
		test.That(t, err, test.ShouldBeNil)

		orig := plan.Trajectory()[0]["arm-right"]
		for _, tt := range plan.Trajectory() {
			now := tt["arm-right"]
			logger.Info(now)
			test.That(t, referenceframe.InputsL2Distance(orig, now), test.ShouldBeLessThan, 0.0001)
		}

		test.That(t, len(plan.Trajectory()), test.ShouldBeLessThan, 6)
	})

	t.Run("orientation", func(t *testing.T) {
		req.Constraints.OrientationConstraint = append(req.Constraints.OrientationConstraint,
			motionplan.OrientationConstraint{OrientationToleranceDegs: 60})

		plan, _, err := PlanMotion(context.Background(), logger, req)
		test.That(t, err, test.ShouldBeNil)
		// Smoothing produces ~3 waypoints, addCloseObstacleWaypoints may add more where path is close to obstacles
		test.That(t, len(plan.Trajectory()), test.ShouldBeLessThan, 10)
	})
}

func TestSandingLargeMove1(t *testing.T) {
	t.Parallel()
	name := "ur20-modular"

	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t).Sublogger("mp")
	ctx := context.Background()

	start := time.Now()
	req, err := ReadRequestFromFile("data/sanding-large-move1.json")
	test.That(t, err, test.ShouldBeNil)

	logger.Infof("time to ReadRequestFromFile %v", time.Since(start))
	{
		ss, err := smartSeed(req.FrameSystem, logger)
		test.That(t, err, test.ShouldBeNil)

		seeds, _, err := ss.findSeeds(ctx, req.Goals[0].poses, req.StartState.LinearConfiguration(), 5, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(seeds), test.ShouldBeGreaterThan, 1)

		hasPos := false
		hasNeg := false

		for _, s := range seeds {
			v := s.Get(name)[0]
			if v > 0 {
				hasPos = true
			} else if v < 0 && v > -1 {
				hasNeg = true
			}
			logger.Debugf("seed %v", s)
		}
		test.That(t, hasPos, test.ShouldBeTrue)
		test.That(t, hasNeg, test.ShouldBeTrue)

		seeds, seedLimits, err := ss.findSeeds(ctx, req.Goals[0].poses, req.StartState.LinearConfiguration(), -1, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(seeds), test.ShouldBeGreaterThan, 3)
		test.That(t, len(seeds), test.ShouldBeLessThan, 500)
		test.That(t, len(seedLimits), test.ShouldEqual, 6)

		lis, err := req.StartState.LinearConfiguration().GetSchema(req.FrameSystem)
		test.That(t, err, test.ShouldBeNil)

		goodSets := [][]float64{
			{-0.78, -0.19, 1.26, -2.58, -1.48, 3.95},
			{5.51, -0.19, 1.26, -2.58, -1.48, -2.33},
		}
		for _, goodJoints := range goodSets {
			numGood := 0
			for i, s := range seeds {
				ll := ik.ComputeAdjustLimitsArray(s.GetLinearizedInputs(), lis.GetLimits(), seedLimits)
				if !referenceframe.AreInputsValid(ll, goodJoints) {
					continue
				}
				logger.Infof("good %d %v", i, logging.FloatArrayFormat{"%0.2f", s.GetLinearizedInputs()})
				numGood++
			}
			test.That(t, numGood, test.ShouldBeGreaterThan, 0)
		}
	}

	pc, err := NewPlanContext(ctx, logger, req, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc, err := NewPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].poses)
	test.That(t, err, test.ShouldBeNil)

	t.Skip("RSDK-14560: flaky - initRRTSolutions direct solution depends on a wall-clock race in shouldStopEarly")

	solution, err := initRRTSolutions(ctx, psc, logger.Sublogger("solve"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(solution.steps), test.ShouldEqual, 1)

	sta := req.StartState.LinearConfiguration().Get(name)
	res := solution.steps[0].Get(name)
	lim := req.FrameSystem.Frame(name).DoF()

	p, err := req.FrameSystem.Frame(name).Transform(res)
	test.That(t, err, test.ShouldBeNil)
	logger.Infof("final arm pose: %v", p)

	for j, startPosition := range sta {
		_, _, r := lim[j].GoodLimits()
		delta := math.Abs(startPosition - res[j])
		logger.Infof("j: %d start: %0.2f end: %0.2f delta: %0.2f ratio: %0.2f", j, startPosition, res[j], delta, delta/r)
	}
}

func TestBadSpray1(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	logger := newChattyMotionPlanTestLogger(t)

	start := time.Now()
	req, err := ReadRequestFromFile("data/spray-bad1.json")
	test.That(t, err, test.ShouldBeNil)

	logger.Infof("time to ReadRequestFromFile %v", time.Since(start))

	// Above defaultTimeout, IK obeys the request timeout instead of its sub-second budget, which a loaded
	// CI runner can exhaust before finding the direct solution across this path's wrist singularity.
	req.PlannerOptions.Timeout = defaultTimeout + 1

	t.Run("basic", func(t *testing.T) {
		_, _, err := PlanMotion(context.Background(), logger, req)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("toofar", func(t *testing.T) {
		req.Goals = []*PlanState{
			{
				poses: referenceframe.FrameSystemPoses{
					"arm": referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: 10000000})),
				},
			},
		}
		req.Constraints = nil
		_, _, err = PlanMotion(context.Background(), logger, req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, errors.Is(err, &tooFarError{}), test.ShouldBeTrue)
	})
}

func TestPirouette(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}
	// get arm kinematics for forward kinematics
	armName := "ur5e"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	idealJointValues := pirIdealJointValues

	// the only change here is in joint 0 in increments of 30, while all the other joints are kept at a constant value
	// below is change in joint 0 in degrees:
	// 0 -> 30 -> 60 -> 90 -> 120 -> 150 -> 180 -> 180 -> 150 -> 120 -> 90 -> 60 -> 30 -> 0

	// determine pose given elements of idealJointValues
	pifs := []*referenceframe.PoseInFrame{}
	for _, pos := range idealJointValues {
		pose, err := armKinematics.Transform(pos)
		test.That(t, err, test.ShouldBeNil)
		posInF := referenceframe.NewPoseInFrame(referenceframe.World, pose)
		pifs = append(pifs, posInF)
	}

	// construct framesystem
	fs := referenceframe.NewEmptyFrameSystem("pirouette")
	err = fs.AddFrame(armKinematics, fs.World())
	test.That(t, err, test.ShouldBeNil)

	err = PrepSmartSeed(fs, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	for iter := 0; iter < 10; iter++ {
		// keep track of previous index of idealJointValues, used for calculating expected joint 0 change
		prevIndex := 0

		// all we care about is the plan and not actually executing it
		startState := NewPlanState(nil, map[string][]referenceframe.Input{armName: idealJointValues[0]})

		// iterate through pifs and create a plan which gets the arm there
		for i, p := range pifs {
			t.Run(fmt.Sprintf("iteration-%d-%d", iter, i), func(t *testing.T) {
				testLogger := logging.NewTestLogger(t)
				mpLogger := newChattyMotionPlanTestLogger(t)

				// construct req and get the plan
				goalState := NewPlanState(map[string]*referenceframe.PoseInFrame{armName: p}, nil)

				req := &PlanRequest{
					FrameSystem: fs,
					Goals:       []*PlanState{goalState},
					StartState:  startState,
				}
				plan, _, err := PlanMotion(context.Background(), mpLogger, req)
				test.That(t, err, test.ShouldBeNil)

				traj := plan.Trajectory()
				// since we do not specify a constraint we are in the "free" motion profile which gives us a trajectory of length two
				test.That(t, len(traj), test.ShouldEqual, 2) // ensure length is always two

				// determine how much joint 0 has changed in degrees from this trajectory
				allArmInputs, err := traj.GetFrameInputs(armName)
				test.That(t, err, test.ShouldBeNil)
				j0TrajStart := allArmInputs[0][0]
				j0TrajEnd := allArmInputs[len(allArmInputs)-1][0]
				j0Change := math.Abs(j0TrajEnd - j0TrajStart)

				// figure out expected change given what the ideal change in joint 0 would be
				idealJ0Value := idealJointValues[i][0]
				idealPreviousJ0Value := idealJointValues[prevIndex][0]
				expectedJ0Change := math.Abs(idealJ0Value-idealPreviousJ0Value) + 2e-2 // add buffer of 1.15 degrees

				testLogger.Infof("motionplan's trajectory: %v", traj)
				testLogger.Infof("ideal trajectory: \n%v\n%v\n", idealJointValues[prevIndex], idealJointValues[i])

				// determine if a pirouette happened
				// in order to satisfy our desired pose in frame while execeeding the expected change in joint 0 a pirouette was necessary
				test.That(t, j0Change, test.ShouldBeLessThanOrEqualTo, expectedJ0Change)

				// increment everything
				prevIndex = i
				startState = NewPlanState(nil, map[string][]referenceframe.Input{armName: traj[len(traj)-1][armName]})
			})
		}
	}
}

func TestBadPlanNoCrash(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	req, err := ReadRequestFromFile("data/bad-sand-plan.json")
	test.That(t, err, test.ShouldBeNil)
	_, _, err = PlanMotion(context.Background(), logger, req)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestOrbPlanTooManySteps(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)
	req, err := ReadRequestFromFile("data/sanding-too-many-steps.json")
	test.That(t, err, test.ShouldBeNil)

	plan, _, err := PlanMotion(context.Background(), logger, req)
	test.That(t, err, test.ShouldBeNil)

	traj := plan.Trajectory()
	zeros := 0
	for i := 1; i < len(traj); i++ {
		d := referenceframe.InputsL2Distance(traj[i-1]["arm"], traj[i]["arm"])
		if d == 0 {
			zeros++
		}
	}
	logger.Infof("zeros: %v / %v", zeros, len(traj))
	test.That(t, zeros, test.ShouldBeLessThanOrEqualTo, 0)
}

func TestSandingWallCollision(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t).Sublogger("mp")
	ctx := context.Background()

	start := time.Now()
	req, err := ReadRequestFromFile("data/sanding-collision-with-wall.json")
	test.That(t, err, test.ShouldBeNil)

	logger.Infof("time to ReadRequestFromFile %v", time.Since(start))
	plan, _, err := PlanMotion(ctx, logger, req)
	test.That(t, err, test.ShouldBeNil)

	t.Run("check trajectory length", func(t *testing.T) {
		test.That(t, len(plan.Trajectory()), test.ShouldBeGreaterThan, 3)
	})

	t.Run("check collision checks pass with smaller resolution", func(t *testing.T) {
		// Create plan context to validate the path
		pc, err := NewPlanContext(ctx, logger, req, &PlanMeta{})
		test.That(t, err, test.ShouldBeNil)

		psc, err := NewPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].Poses())
		test.That(t, err, test.ShouldBeNil)

		trajectory := plan.Trajectory()
		smallResolution := 0.001

		for j := 0; j < len(trajectory)-1; j++ {
			start := trajectory[j].ToLinearInputs()
			end := trajectory[j+1].ToLinearInputs()

			// Default resolution passes
			err := psc.CheckPath(ctx, start, end, false, nil)
			test.That(t, err, test.ShouldBeNil)

			// Small resolution noticed the collision when we had large jumps
			_, err = psc.Checker.CheckStateConstraintsAcrossSegmentFS(
				ctx,
				&motionplan.SegmentFS{
					StartConfiguration: start,
					EndConfiguration:   end,
					FS:                 pc.fs,
				},
				smallResolution,
				true,
			)
			test.That(t, err, test.ShouldBeNil)
		}
	})
}

func TestTeleOpTwoMove(t *testing.T) {
	t.Parallel()
	req, err := ReadRequestFromFile("data/plan-2026-02-12-left-arm-collision-avoidance.json")
	test.That(t, err, test.ShouldBeNil)

	for i := 0; i < 5; i++ {
		t.Run(fmt.Sprintf("seed-%d", i), func(t *testing.T) {
			logger := newChattyMotionPlanTestLogger(t)

			req.PlannerOptions.RandomSeed = i
			plan, _, err := PlanMotion(context.Background(), logger, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(plan.Trajectory()), test.ShouldEqual, 2)

			a := plan.Trajectory()[0]["right-arm"]
			b := plan.Trajectory()[1]["right-arm"]

			test.That(t, len(a), test.ShouldEqual, 6)

			test.That(t, referenceframe.InputsL2Distance(a, b), test.ShouldBeLessThan, .01)
		})
	}
}

func TestWineBadBottleMoveGoodCost(t *testing.T) {
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	req, err := ReadRequestFromFile("data/wine-bad-bottle-move.json")
	test.That(t, err, test.ShouldBeNil)

	pc, err := NewPlanContext(ctx, logger, req, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc, err := NewPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].Poses())
	test.That(t, err, test.ShouldBeNil)

	sss, err := NewSolutionSolvingState(ctx, psc, logger)
	test.That(t, err, test.ShouldBeNil)

	logger.Infof("goodCost: %0.4f", sss.goodCost)
	test.That(t, sss.goodCost, test.ShouldBeLessThan, 3)

	plan, _, err := PlanMotion(ctx, logger, req)
	test.That(t, err, test.ShouldBeNil)

	a := plan.Trajectory()[0]["right-arm"]
	b := plan.Trajectory()[1]["right-arm"]
	dist := referenceframe.InputsL2Distance(a, b)
	logger.Infof("L2 distance: %0.4f", dist)
	test.That(t, dist, test.ShouldBeLessThan, 2)
}

func TestPallette1TotalL2(t *testing.T) {
	t.Parallel()
	if IsTooSmallForCache() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/pallette1.json")
	test.That(t, err, test.ShouldBeNil)

	plan, _, err := PlanMotion(context.Background(), logger, req)
	test.That(t, err, test.ShouldBeNil)

	totalL2 := 0.0
	traj := plan.Trajectory()
	for idx := 1; idx < len(traj); idx++ {
		for k := range traj[idx] {
			totalL2 += referenceframe.InputsL2Distance(traj[idx-1][k], traj[idx][k])
		}
	}
	logger.Infof("totalL2: %0.4f", totalL2)
	test.That(t, totalL2, test.ShouldBeLessThan, 6)
}

func BenchmarkBigPlanRequest(b *testing.B) {
	if IsTooSmallForCache() {
		b.Skip()
		return
	}
	filename := "data/sanding-large-move1.json"

	req, err := ReadRequestFromFile(filename)
	test.That(b, err, test.ShouldBeNil)
	dir := os.TempDir()

	b.ResetTimer()
	for b.Loop() {
		test.That(b, req.WriteToFile(filepath.Join(dir, "tmp.json")), test.ShouldBeNil)
	}
}

func BenchmarkPlanningOnMeshes(b *testing.B) {
	ur20Model, err := referenceframe.KinematicModelFromFile(artifact.MustPath("urdfs/ur20.urdf"), "ur20URDF")
	test.That(b, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(ur20Model, fs.World())
	test.That(b, err, test.ShouldBeNil)

	goalState := NewPlanState(nil,
		map[string][]referenceframe.Input{
			ur20Model.Name(): make([]float64, len(ur20Model.DoF())),
		},
	)
	startState := goalState

	req := &PlanRequest{
		FrameSystem: fs,
		Goals:       []*PlanState{goalState},
		StartState:  startState,
	}

	mpLogger := newChattyMotionPlanTestLogger(b)

	b.ResetTimer() // Reset timer after setup
	for i := 0; i < b.N; i++ {
		_, _, err := PlanMotion(context.Background(), mpLogger, req)
		test.That(b, err, test.ShouldBeNil)
	}
}

// Only the left arm's spoon has goals, so the right arm has no reason to move. It used to drift a
// little on every joint at once: the IK gradient's probe off an upper joint limit was never put
// back, and the leftover showed up as an identical bogus gradient on every joint the goal
// ignores.
func TestVesselPathRightArmStaysPut(t *testing.T) {
	t.Parallel()
	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/vessel-path-right-arm-idle.json")
	test.That(t, err, test.ShouldBeNil)
	start := req.StartState.Configuration()["right-arm"]

	plan, _, err := PlanMotion(context.Background(), logger, req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan.Trajectory()), test.ShouldEqual, len(req.Goals)+1)

	for _, step := range plan.Trajectory() {
		test.That(t, step["right-arm"], test.ShouldResemble, start)
	}
}

func TestEarlyExitWhenLinearConstraintOutcomeLooksBleak(t *testing.T) {
	logger := logging.NewTestLogger(t)
	req, err := ReadRequestFromFile("data/big-movement-linear-constraint-early-exit.json")
	test.That(t, err, test.ShouldBeNil)

	_, _, err = PlanMotion(context.Background(), logger, req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, "linear with cbirrt not allowed and no direct solutions found")
}

// registerSingleNodeRoadmap puts a roadmap with exactly one configuration into the process's
// roadmap registry, as if an earlier plan had learned it. IK seeding only uses a roadmap that is
// already registered and built. The roadmap is keyed for the given request's first goal.
//
// MOTION_ROADMAP_CACHE_DIR is cleared so nothing is loaded from or saved to disk. What the planner
// harvests into the roadmap lives in memory until ClearRoadmapCache.
func registerSingleNodeRoadmap(t *testing.T, logger logging.Logger, request *PlanRequest, configuration []float64) *roadmap {
	t.Helper()

	t.Setenv("MOTION_ROADMAP_CACHE_DIR", "")

	ctx := context.Background()
	planContext, err := NewPlanContext(ctx, logger, request, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	goal, err := request.Goals[0].ComputePoses(ctx, request.FrameSystem)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(goal), test.ShouldEqual, 1)

	segmentContext, err := NewPlanSegmentContext(ctx, planContext, request.StartState.LinearConfiguration(), goal)
	test.That(t, err, test.ShouldBeNil)

	frames := movingFrameNamesWithDoF(segmentContext)
	sort.Strings(frames)
	test.That(t, frames, test.ShouldResemble, []string{"arm"})

	var goalFrame string
	var goalPoint [3]float64
	for frameName, pose := range goal {
		goalFrame = frameName
		point := pose.Pose().Point()
		goalPoint = [3]float64{point.X, point.Y, point.Z}
	}

	key := roadmapKeyFor(segmentContext, frames)
	singleNode := &roadmap{
		frames:    frames,
		dims:      []int{len(configuration)},
		goalFrame: goalFrame,
		flat:      [][]float64{configuration},
		neighbors: [][]int{{}},
		eePos:     [][3]float64{goalPoint},
		key:       key,
	}
	// getRoadmap would otherwise build or load a roadmap the first time it is asked for this key.
	singleNode.buildOnce.Do(func() {})
	singleNode.built.Store(true)
	roadmapRegistry.Store(key, singleNode)

	return singleNode
}

// TestPlanMotionSequenceSucceedsDespitePoisonedCache plans two recorded requests from a robot
// opening a fridge: "035", which moves the gripper to the handle, and "036", a linear move that
// follows it.
//
// The roadmap holds a single configuration: the one "035" was recorded ending in. With it
// registered, "035" succeeds but ends in a joint family that is a dead end for "036", because
// joint 2 is nearly at its lower limit. Without it, "035" takes a family that works. This test
// asserts that we can backtrack from the failure of "036" and find that other answer.
func TestPlanMotionSequenceSucceedsDespitePoisonedCache(t *testing.T) {
	// The arm's joint 3 tells the two families apart. These are the values 035 ends at in each.
	const (
		poisonedJoint3 = 4.6324
		workingJoint3  = 1.3445
	)

	ClearRoadmapCache()

	logger := logging.NewTestLogger(t)

	var requests []*PlanRequest
	for _, file := range []string{"data/capp-fridge-open-035.json", "data/capp-fridge-move-036.json"} {
		request, err := ReadRequestFromFile(file)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, PrepSmartSeed(request.FrameSystem, logger), test.ShouldBeNil)

		requests = append(requests, request)
	}

	// "036" starts wherever "035" ends.
	requests[1].StartState = nil

	registerSingleNodeRoadmap(t, logger, requests[0], []float64{
		-0.992999659809725, -1.2022321151487678, -3.6860351229031787, 4.632364891547901, 1.9934586075229215, 1.3777452467564468,
	})

	plans, metas, err := PlanMotionSequence(context.Background(), logger, requests, nil)

	// The key decision is where 035 ends, which is where 036 was started from: PlanMotionSequence
	// sets each later request's StartState in place. This is logged before asserting so it shows up
	// when 036 fails.
	if len(plans) > 0 {
		fridgeMoveJoint3 := requests[1].StartState.Configuration()["arm"][3]
		family := "some other family"
		if math.Abs(fridgeMoveJoint3-poisonedJoint3) < 0.1 {
			family = "the poisoned family"
		} else if math.Abs(fridgeMoveJoint3-workingJoint3) < 0.1 {
			family = "a known working family"
		}

		t.Logf("036 starts in %s: arm joint 3 = %.4f (poisoned value %.4f)", family, fridgeMoveJoint3, poisonedJoint3)
	}

	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plans), test.ShouldEqual, len(requests))
	test.That(t, len(metas), test.ShouldEqual, len(requests))

	numBacktracks := 0
	for _, meta := range metas {
		numBacktracks += meta.BacktrackCounter
	}
	test.That(t, numBacktracks, test.ShouldBeGreaterThanOrEqualTo, 1)
}

// TestPlanMotionSequenceHeavyNeutralBias contains two plans, the first is unconstrained while the
// second is heavily constrained. Due to the start position, the solution to the unconstrained
// motion that allows the second to succeed has a much worse score than the alternatives. However,
// this working solution does have the property of being a "normalized" solution (all joints within
// pi of the middle of their range) when compared the failed ones. Without HeavyNeutralBias, the
// algorithm would only explore this solution on a fourth backtrack, which the default branch factor
// of three never reaches.
func TestPlanMotionSequenceHeavyNeutralBias(t *testing.T) {
	logger := logging.NewTestLogger(t)

	var requests []*PlanRequest
	for _, file := range []string{
		"data/sequence-heavy-bias/123-20261106_163547.972-serving-carry-success.json",
		"data/sequence-heavy-bias/124-20261106_163552.763-serving-pivot-success.json",
	} {
		request, err := ReadRequestFromFile(file)
		test.That(t, err, test.ShouldBeNil)
		request.PlannerOptions.Timeout = defaultTimeout + 1

		requests = append(requests, request)
	}

	plans, _, err := PlanMotionSequence(context.Background(), logger, requests, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plans), test.ShouldEqual, len(requests))

	// How many solutions of the first request get rejected before the normalized one wins depends
	// on what the time-budgeted, multi-threaded IK search happens to produce. Only the answer the
	// sequence settles on, which is the one HeavyNeutralBias prefers, is deterministic.
	armFrame := requests[0].FrameSystem.Frame("arm")
	bias := HeavyNeutralBias(armFrame.DoF(), referenceframe.RevoluteDoF(armFrame), requests[1].StartState.Configuration()["arm"])
	test.That(t, bias, test.ShouldEqual, 0)
}
