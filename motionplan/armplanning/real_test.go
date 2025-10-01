//go:build !386 && !arm

package armplanning

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

func TestOrbOneSeed(t *testing.T) {
	matches, err := filepath.Glob("data/orb-plan*.json")
	test.That(t, err, test.ShouldBeNil)

	for _, fp := range matches {
		t.Run(fp, func(t *testing.T) {
			logger := logging.NewTestLogger(t)

			req, err := ReadRequestFromFile(fp)
			test.That(t, err, test.ShouldBeNil)

			plan, err := PlanMotion(context.Background(), logger, req)
			test.That(t, err, test.ShouldBeNil)

			a := plan.Trajectory()[0]["sanding-ur5"]
			b := plan.Trajectory()[1]["sanding-ur5"]

			test.That(t, referenceframe.InputsL2Distance(a, b), test.ShouldBeLessThan, .005)
		})
	}
}

func TestOrbManySeeds(t *testing.T) {
	matches, err := filepath.Glob("data/orb-plan*.json")
	test.That(t, err, test.ShouldBeNil)

	for _, fp := range matches {
		req, err := ReadRequestFromFile(fp)
		test.That(t, err, test.ShouldBeNil)

		for i := 0; i < 100; i++ {
			t.Run(fmt.Sprintf("%s-%d", fp, i), func(t *testing.T) {
				logger := logging.NewTestLogger(t)

				req.PlannerOptions.RandomSeed = i
				plan, err := PlanMotion(context.Background(), logger, req)
				test.That(t, err, test.ShouldBeNil)

				a := plan.Trajectory()[0]["sanding-ur5"]
				b := plan.Trajectory()[1]["sanding-ur5"]

				test.That(t, referenceframe.InputsL2Distance(a, b), test.ShouldBeLessThan, .005)
			})
		}
	}
}

func TestPourManySeeds(t *testing.T) {
	req, err := ReadRequestFromFile("data/pour-plan-bad.json")
	test.That(t, err, test.ShouldBeNil)

	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("seed-%d", i), func(t *testing.T) {
			logger := logging.NewTestLogger(t)

			req.PlannerOptions.RandomSeed = i
			plan, err := PlanMotion(context.Background(), logger, req)
			test.That(t, err, test.ShouldBeNil)

			a := plan.Trajectory()[0]["arm-right"]
			b := plan.Trajectory()[1]["arm-right"]

			test.That(t, referenceframe.InputsL2Distance(a, b), test.ShouldBeLessThan, .15)
		})
	}
}

func TestWineCrazyTouch1(t *testing.T) {
	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/wine-crazy-touch.json")
	test.That(t, err, test.ShouldBeNil)

	plan, err := PlanMotion(context.Background(), logger, req)
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
	logger := logging.NewTestLogger(t)

	req, err := ReadRequestFromFile("data/wine-crazy-touch2.json")
	test.That(t, err, test.ShouldBeNil)

	plan, err := PlanMotion(context.Background(), logger, req)
	test.That(t, err, test.ShouldBeNil)

	orig := plan.Trajectory()[0]["arm-right"]
	for _, tt := range plan.Trajectory() {
		now := tt["arm-right"]
		logger.Info(now)
		test.That(t, referenceframe.InputsL2Distance(orig, now), test.ShouldBeLessThan, 0.0001)
	}

	test.That(t, len(plan.Trajectory()), test.ShouldBeLessThan, 6)
}

func TestSandingLargeMove1(t *testing.T) {
	logger := logging.NewTestLogger(t)

	start := time.Now()
	req, err := ReadRequestFromFile("data/sanding-large-move1.json")
	test.That(t, err, test.ShouldBeNil)

	logger.Infof("time to ReadRequestFromFile %v", time.Since(start))

	pc, err := newPlanContext(logger, req)
	test.That(t, err, test.ShouldBeNil)

	psc, err := newPlanSegmentContext(pc, req.StartState.configuration, req.Goals[0].poses)
	test.That(t, err, test.ShouldBeNil)

	solution, err := initRRTSolutions(context.Background(), psc)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(solution.steps), test.ShouldEqual, 1)
}

func TestPirouette(t *testing.T) {
	t.Skip()

	// get arm kinematics for forward kinematics
	armName := "ur5e"
	armKinematics, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), armName)
	test.That(t, err, test.ShouldBeNil)

	idealJointValues := [][]referenceframe.Input{
		{{0 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{30 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{60 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{90 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{120 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{150 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{180 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{180 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{150 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{120 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{90 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{60 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{30 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
		{{0 * 3.1415 / 180.0}, {0}, {-90 * 3.1415 / 180.0}, {0}, {0}, {0}},
	}
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

	for iter := 0; iter < 10; iter++ {
		// keep track of previous index of idealJointValues, used for calculating expected joint 0 change
		prevIndex := 0

		// all we care about is the plan and not actually executing it
		startState := NewPlanState(nil, map[string][]referenceframe.Input{armName: idealJointValues[0]})

		// iterate through pifs and create a plan which gets the arm there
		for i, p := range pifs {
			t.Run(fmt.Sprintf("iteration: %d-%d", iter, i), func(t *testing.T) {
				logger := logging.NewTestLogger(t)
				// construct req and get the plan
				goalState := NewPlanState(map[string]*referenceframe.PoseInFrame{armName: p}, nil)
				req := &PlanRequest{
					FrameSystem: fs,
					Goals:       []*PlanState{goalState},
					StartState:  startState,
				}
				plan, err := PlanMotion(context.Background(), logger, req)
				test.That(t, err, test.ShouldBeNil)

				traj := plan.Trajectory()
				// since we do not specify a constraint we are in the "free" motion profile which gives us a trajectory of length two
				test.That(t, len(traj), test.ShouldEqual, 2) // ensure length is always two

				// determine how much joint 0 has changed in degrees from this trajectory
				allArmInputs, err := traj.GetFrameInputs(armName)
				test.That(t, err, test.ShouldBeNil)
				j0TrajStart := allArmInputs[0][0].Value
				j0TrajEnd := allArmInputs[len(allArmInputs)-1][0].Value
				j0ChangeDeg := utils.RadToDeg(math.Abs(j0TrajEnd - j0TrajStart))

				// figure out expected change given what the ideal change in joint 0 would be
				idealJ0Value := idealJointValues[i][0].Value
				idealPreviousJ0Value := idealJointValues[prevIndex][0].Value
				expectedJ0ChangeDeg := utils.RadToDeg(math.Abs(idealJ0Value-idealPreviousJ0Value)) + 1e-1 // add buffer

				logger.Infof("motionplan's trajectory: %v", traj)
				logger.Infof("ideal trajectory: \n%v\n%v\n", idealJointValues[prevIndex], idealJointValues[i])

				// determine if a pirouette happened
				// in order to satisfy our desired pose in frame while execeeding the expected change in joint 0 a pirouette was necessary
				test.That(t, j0ChangeDeg, test.ShouldBeLessThanOrEqualTo, expectedJ0ChangeDeg)

				// increment everything
				prevIndex = i
				startState = NewPlanState(nil, map[string][]referenceframe.Input{armName: traj[len(traj)-1][armName]})
			})
		}
	}
}
