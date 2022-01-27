package oneAxis

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

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

func TestNewoneAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakeRobot := createFakeRobot()

	fakecfg := config.Component{
		Name: "gantry",
		Attributes: config.AttributeMap{
			"motor":           "x",
			"limitSwitchPins": []string{"1", "2"},
			"length_mm":       1.0,
			"board":           "board",
			"limitHigh":       true,
			"rpm":             float64(300),
		},
	}

	fakegantry, err := NewOneAxis(ctx, fakeRobot, fakecfg, logger)
	realG, ok := fakegantry.(*oneAxis)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, realG.limitType, test.ShouldEqual, "twoPin")

	fakecfg = config.Component{
		Name: "gantry",
		Attributes: config.AttributeMap{
			"motor":           "x",
			"limitSwitchPins": []string{"1"},
			"length_mm":       1.0,
			"board":           "board",
			"limitHigh":       true,
			"rpm":             float64(300),
		},
	}
	fakegantry, err = NewOneAxis(ctx, fakeRobot, fakecfg, logger)
	realG, ok = fakegantry.(*oneAxis)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, realG.limitType, test.ShouldEqual, "onePinOneLength")

	fakecfg = config.Component{
		Name: "gantry",
		Attributes: config.AttributeMap{
			"motor":           "x",
			"limitSwitchPins": []string{},
			"length_mm":       0.0,
			"board":           "board",
		},
	}
	_, err = NewOneAxis(ctx, fakeRobot, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "gantry length has to be >= 0")

	shamcfg := config.Component{
		Attributes: config.AttributeMap{
			"motorList":       []string{},
			"limitSwitchPins": []string{},
			"lengthMeters":    []float64{},
			"board":           "",
		},
	}
	_, err := NewOneAxis(ctx, shamRobot, shamcfg, logger)
	test.That(t, err, test.ShouldNotBeNil)

	shamcfg = config.Component{
		Attributes: config.AttributeMap{
			"motorList":       []string{"x", "y", "z"},
			"limitSwitchPins": []string{"1", "2", "3", "4", "5", "6"},
			"lengthMeters":    []float64{1.0, 1.0, 1.0},
			"board":           "board",
		},
	}

	_, err = NewOneAxis(ctx, shamRobot, shamcfg, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestInit(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &oneAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
		limitType:       "onePinOneLength",
	}

	err := fakegantry.init(ctx)
	test.That(t, err, test.ShouldBeNil)

	fakegantry = &oneAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
		limitType:       "twoPin",
	}
	err = fakegantry.init(ctx)
	test.That(t, err, test.ShouldBeNil)

	fakegantry = &oneAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
		limitType:       "encoder",
	}
	err = fakegantry.init(ctx)
	test.That(t, err, test.ShouldNotBeNil)

}

func TestHomeTwoLimitSwitch(t *testing.T) {
	motor := createShamMotor()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &oneAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2", "3", "4", "5", "6"},
	}

	err := fakegantry.homeTwoLimSwitch(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestHomeOneLimitSwitch(t *testing.T) {
	motor := createShamMotor()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &oneAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2", "3"},
		length_mm:       float64(1),
		pulleyR_mm:      float64(.1),
	}

	err := fakegantry.homeOneLimSwitch(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestHomeEncoder(t *testing.T) {
	fakegantry := &oneAxis{}
	ctx := context.Background()
	err := fakegantry.homeEncoder(ctx)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestTestLimit(t *testing.T) {
	ctx := context.Background()
	fakegantry := &oneAxis{
		limitSwitchPins: []string{"1", "2"},
		motor:           createShamMotor(),
		limitBoard:      createFakeBoard(),
		rpm:             float64(300),
		limitHigh:       true,
	}
	pos, err := fakegantry.testLimit(ctx, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, float64(1))
}

func TestLimitHit(t *testing.T) {
	ctx := context.Background()
	fakegantry := &oneAxis{
		limitSwitchPins: []string{"1", "2", "3"},
		limitBoard:      createFakeBoard(),
		limitHigh:       true,
	}

	hit, err := fakegantry.limitHit(ctx, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, hit, test.ShouldEqual, true)
	test.That(t, err, test.ShouldBeNil)
}

func TestCurrentPosition(t *testing.T) {
	logger := golog.NewTestLogger(t)

	fakemotor := createShamMotor()
	ctx := context.Background()
	fakegantry := &oneAxis{
		board:           createFakeBoard(),
		limitHigh:       true,
		motor:           fakemotor,
		limitSwitchPins: []string{"1", "2", "3", "4", "5", "6"},
		positionLimits:  []float64{0, 1, 0, 1, 0, 1},
		logger:          logger,
	}
	positions, err := fakegantry.CurrentPosition(ctx)

	test.That(t, positions, test.ShouldResemble, []float64{1, 1, 1})
	test.That(t, err, test.ShouldBeNil)
}

func TestLengths(t *testing.T) {
	fakegantry := &oneAxis{
		length_mm: float64(1.0),
	}
	ctx := context.Background()
	fakelengths, err := fakegantry.Lengths(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.ShouldHaveLength(t, fakelengths, test.ShouldEqual(float64(1.0)))
}

// TODO: tests for reference frame
