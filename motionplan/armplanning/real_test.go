package armplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

func readRequestFromFile(f string) (*PlanRequest, error) {
	content, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}

	req := &PlanRequest{}

	err = json.Unmarshal(content, req)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func TestOrbOneSeed(t *testing.T) {
	matches, err := filepath.Glob("data/orb-plan*.json")
	test.That(t, err, test.ShouldBeNil)

	for _, fp := range matches {
		t.Run(fp, func(t *testing.T) {
			logger := logging.NewTestLogger(t)

			req, err := readRequestFromFile(fp)
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
		req, err := readRequestFromFile(fp)
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
	req, err := readRequestFromFile("data/pour-plan-bad.json")
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

	req, err := readRequestFromFile("data/wine-crazy-touch.json")
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

	req, err := readRequestFromFile("data/wine-crazy-touch2.json")
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
