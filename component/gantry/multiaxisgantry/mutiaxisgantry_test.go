package multiaxisgantry

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/testutils/inject"
)

func createShamMotor() *inject.Motor {
	shamMotor := &inject.Motor{}

	shamMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}

	shamMotor.PositionFunc = func(ctx context.Context) (float64, error) {
		return 1, nil
	}

	shamMotor.GoForFunc = func(ctx context.Context, rpm float64, revolutions float64) error {
		return nil
	}

	shamMotor.StopFunc = func(ctx context.Context) error {
		return nil
	}

	shamMotor.GoFunc = func(ctx context.Context, powerPct float64) error {
		return nil
	}

	return shamMotor
}

func createFakeBoard() *inject.Board {
	fakeboard := &inject.Board{}

	fakeboard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
		return true, nil
	}
	return fakeboard
}

func TestNewMultiAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	shamRobot := &inject.Robot{}

	shamcfg := config.Component{
		Attributes: config.AttributeMap{
			"motorList":       []string{},
			"limitSwitchPins": []string{},
			"lengthMeters":    []float64{},
			"board":           "",
		},
	}
	_, err := NewMultiAxis(ctx, shamRobot, shamcfg, logger)
	test.That(t, err, test.ShouldNotBeNil)

	shamcfg = config.Component{
		Attributes: config.AttributeMap{
			"motorList":       []string{"x", "y", "z"},
			"limitSwitchPins": []string{"1", "2", "3", "4", "5", "6"},
			"lengthMeters":    []float64{1.0, 1.0, 1.0},
			"board":           "board",
		},
	}

	_, err = NewMultiAxis(ctx, shamRobot, shamcfg, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestInit(t *testing.T) {
	ctx := context.Background()
	fakemotor := createShamMotor()
	_, err := fakemotor.PositionSupportedFunc(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestHomeTwoLimitSwitch(t *testing.T) {
	motors := []motor.Motor{createShamMotor(), createShamMotor(), createShamMotor()}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &multiAxis{
		motorList:       []string{"x", "y", "z"},
		motors:          motors,
		limitBoard:      createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             []float64{300, 300, 300},
		limitSwitchPins: []string{"1", "2", "3", "4", "5", "6"},
	}

	err := fakegantry.homeTwoLimSwitch(ctx, 0, []int{0, 1})
	test.That(t, err, test.ShouldBeNil)
}

func TestHomeOneLimitSwitch(t *testing.T) {
	motors := []motor.Motor{createShamMotor(), createShamMotor(), createShamMotor()}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &multiAxis{
		motorList:       []string{"x", "y", "z"},
		motors:          motors,
		limitBoard:      createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             []float64{300, 300, 300},
		limitSwitchPins: []string{"1", "2", "3"},
		lengthMeters:    []float64{1, 1, 1},
		pulleyR:         []float64{.1, .1, .1},
	}

	err := fakegantry.homeOneLimSwitch(ctx, 0, []int{0, 1})
	test.That(t, err, test.ShouldBeNil)
}

func TestHomeEncoder(t *testing.T) {
	fakegantry := &multiAxis{}
	ctx := context.Background()
	err := fakegantry.homeEncoder(ctx, 1)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestTestLimit(t *testing.T) {
	ctx := context.Background()
	fakegantry := &multiAxis{
		limitSwitchPins: []string{"1", "2"},
		motors:          []motor.Motor{createShamMotor()},
		limitBoard:      createFakeBoard(),
		rpm:             []float64{300},
		limitHigh:       true,
	}
	pos, err := fakegantry.testLimit(ctx, 0, []int{0, 1}, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, float64(1))
}

func TestLimitHit(t *testing.T) {
	ctx := context.Background()
	fakegantry := &multiAxis{
		limitSwitchPins: []string{"1", "2", "3"},
		limitBoard:      createFakeBoard(),
		limitHigh:       true,
	}

	hit, err := fakegantry.limitHit(ctx, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, hit, test.ShouldEqual, true)
	test.That(t, err, test.ShouldBeNil)
}

func TestCurrentPosition(t *testing.T) {
	logger := golog.NewTestLogger(t)

	fakemotors := []motor.Motor{createShamMotor(), createShamMotor(), createShamMotor()}
	ctx := context.Background()
	fakegantry := &multiAxis{
		limitBoard:      createFakeBoard(),
		limitHigh:       true,
		motorList:       []string{"x", "y", "z"},
		motors:          fakemotors,
		limitSwitchPins: []string{"1", "2", "3", "4", "5", "6"},
		positionLimits:  []float64{0, 1, 0, 1, 0, 1},
		logger:          logger,
	}
	positions, err := fakegantry.CurrentPosition(ctx)

	test.That(t, positions, test.ShouldResemble, []float64{1, 1, 1})
	test.That(t, err, test.ShouldBeNil)
}

func TestLengths(t *testing.T) {
	fakegantry := &multiAxis{
		motorList:    []string{"x", "y", "z"},
		lengthMeters: []float64{1.0, 2.0, 3.0},
	}
	ctx := context.Background()
	fakelengths, err := fakegantry.Lengths(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.ShouldHaveLength(t, fakelengths, len(fakegantry.motorList))
}

// TODO: tests for reference frame
