//go:build !386

package armplanning

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestOrbOneSeed(t *testing.T) {
	if Is32Bit() {
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
	if Is32Bit() {
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
				logger := logging.NewTestLogger(t)

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
	if Is32Bit() {
		t.Skip()
		return
	}

	req, err := ReadRequestFromFile("data/pour-plan-bad.json")
	test.That(t, err, test.ShouldBeNil)

	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("seed-%d", i), func(t *testing.T) {
			logger := logging.NewTestLogger(t)

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
	if Is32Bit() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/wine-crazy-touch.json")
	test.That(t, err, test.ShouldBeNil)

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
	if Is32Bit() {
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

		test.That(t, len(plan.Trajectory()), test.ShouldBeLessThan, 6)
	})

	t.Run("orientation", func(t *testing.T) {
		req.Constraints.OrientationConstraint = append(req.Constraints.OrientationConstraint,
			motionplan.OrientationConstraint{60})

		plan, _, err := PlanMotion(context.Background(), logger, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(plan.Trajectory()), test.ShouldBeLessThan, 6)
	})
}

func TestSandingLargeMove1(t *testing.T) {
	name := "ur20-modular"

	if Is32Bit() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)
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

		seeds, _, err = ss.findSeeds(ctx, req.Goals[0].poses, req.StartState.LinearConfiguration(), -1, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(seeds), test.ShouldBeGreaterThan, 5)
		test.That(t, len(seeds), test.ShouldBeLessThan, 5000)
	}

	pc, err := newPlanContext(ctx, logger, req, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc, err := newPlanSegmentContext(ctx, pc, req.StartState.LinearConfiguration(), req.Goals[0].poses)
	test.That(t, err, test.ShouldBeNil)

	solution, err := initRRTSolutions(context.Background(), psc, logger.Sublogger("ik"))
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
	if Is32Bit() {
		t.Skip()
		return
	}

	logger := logging.NewTestLogger(t)

	start := time.Now()
	req, err := ReadRequestFromFile("data/spray-bad1.json")
	test.That(t, err, test.ShouldBeNil)

	logger.Infof("time to ReadRequestFromFile %v", time.Since(start))

	t.Run("basic", func(t *testing.T) {
		_, _, err = PlanMotion(context.Background(), logger, req)
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
	if Is32Bit() {
		t.Skip()
		return
	}
	// get arm kinematics for forward kinematics
	armName := "ur5e"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
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
				logger := logging.NewTestLogger(t)
				// construct req and get the plan
				goalState := NewPlanState(map[string]*referenceframe.PoseInFrame{armName: p}, nil)

				req := &PlanRequest{
					FrameSystem: fs,
					Goals:       []*PlanState{goalState},
					StartState:  startState,
				}
				plan, _, err := PlanMotion(context.Background(), logger, req)
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

				logger.Infof("motionplan's trajectory: %v", traj)
				logger.Infof("ideal trajectory: \n%v\n%v\n", idealJointValues[prevIndex], idealJointValues[i])

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
