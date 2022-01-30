package oneaxis

import (
	"context"
	"errors"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/testutils/inject"
)

func createFakeMotor() *inject.Motor {
	fakeMotor := &inject.Motor{}

	fakeMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}

	fakeMotor.PositionFunc = func(ctx context.Context) (float64, error) {
		return 1, nil
	}

	fakeMotor.GoForFunc = func(ctx context.Context, rpm float64, revolutions float64) error {
		return nil
	}

	fakeMotor.StopFunc = func(ctx context.Context) error {
		return nil
	}

	fakeMotor.GoFunc = func(ctx context.Context, powerPct float64) error {
		return nil
	}

	return fakeMotor
}

func createFakeBoard() *inject.Board {
	fakeboard := &inject.Board{}

	fakeboard.GetGPIOFunc = func(ctx context.Context, pin string) (bool, error) {
		return true, nil
	}
	return fakeboard
}

func createFakeRobot() *inject.Robot {
	fakerobot := &inject.Robot{}

	fakerobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return &fake.Motor{PositionSupportedFunc: true, GoForfunc: true}, true
	}

	fakerobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return &inject.Board{GetGPIOFunc: func(ctx context.Context, pin string) (bool, error) {
			return true, nil
		}}, true
	}
	return fakerobot
}

func TestNewOneAxis(t *testing.T) {
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

	fakecfg = config.Component{
		Name: "gantry",
		Attributes: config.AttributeMap{
			"motor":           "x",
			"limitSwitchPins": []string{},
			"length_mm":       1.0,
			"board":           "board",
		},
	}

	_, err = NewOneAxis(ctx, fakeRobot, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "encoder currently not supported")

	fakecfg = config.Component{
		Name: "gantry",
		Attributes: config.AttributeMap{
			"motor":           "x",
			"limitSwitchPins": []string{"1", "2", "3"},
			"length_mm":       1.0,
			"board":           "board",
		},
	}

	_, err = NewOneAxis(ctx, fakeRobot, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid gantry type: need 1, 2 or 0 pins per axis, have 3 pins")
}

func TestHome(t *testing.T) {
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

	err := fakegantry.Home(ctx)
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
	err = fakegantry.Home(ctx)
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
	err = fakegantry.Home(ctx)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestHomeTwoLimitSwitch(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &oneAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
	}

	err := fakegantry.homeTwoLimSwitch(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakegantry.positionLimits, test.ShouldResemble, []float64{1, 1})
}

func TestHomeOneLimitSwitch(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &oneAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1"},
		lengthMm:        float64(1),
		pulleyRMm:       float64(.1),
	}

	err := fakegantry.homeOneLimSwitch(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakegantry.positionLimits, test.ShouldResemble, []float64{1, 2.5915494309189535})
}

func TestHomeEncoder(t *testing.T) {
	fakegantry := &oneAxis{}
	ctx := context.Background()
	err := fakegantry.homeEncoder(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, "encoder currently not supported")
}

func TestTestLimit(t *testing.T) {
	ctx := context.Background()
	fakegantry := &oneAxis{
		limitSwitchPins: []string{"1", "2"},
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
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
		board:           createFakeBoard(),
		limitHigh:       true,
	}

	hit, err := fakegantry.limitHit(ctx, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, hit, test.ShouldEqual, true)
	test.That(t, err, test.ShouldBeNil)
}

func TestGetPosition(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	fakegantry := &oneAxis{
		motor: &inject.Motor{
			PositionSupportedFunc: func(ctx context.Context) (bool, error) { return false, nil },
			PositionFunc:          func(ctx context.Context) (float64, error) { return 1, nil },
		},
		board:           createFakeBoard(),
		positionLimits:  []float64{0, 1},
		limitHigh:       true,
		limitSwitchPins: []string{"1", "2"},
		limitType:       switchLimitTypetwoPin,
		logger:          logger,
	}
	positions, err := fakegantry.GetPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, positions, test.ShouldResemble, []float64{0})

	fakegantry = &oneAxis{
		motor: &inject.Motor{
			PositionSupportedFunc: func(ctx context.Context) (bool, error) { return false, errors.New("not supported") },
			PositionFunc:          func(ctx context.Context) (float64, error) { return 1, errors.New("not supported") },
		},
		board:           createFakeBoard(),
		limitHigh:       true,
		limitSwitchPins: []string{"1", "2"},
		limitType:       switchLimitTypetwoPin,
		positionLimits:  []float64{0, 1},
		logger:          logger,
	}
	positions, err = fakegantry.GetPosition(ctx)
	test.That(t, positions, test.ShouldResemble, []float64{})
	test.That(t, err, test.ShouldNotBeNil)
}

func TestGetLengths(t *testing.T) {
	fakegantry := &oneAxis{
		lengthMm: float64(1.0),
	}
	ctx := context.Background()
	fakelengths, err := fakegantry.GetLengths(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.ShouldHaveLength(t, fakelengths, test.ShouldEqual(float64(1.0)))
}

func TestMoveToPosition(t *testing.T) {
	// TODO
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &oneAxis{
		logger:    logger,
		board:     createFakeBoard(),
		motor:     createFakeMotor(),
		limitHigh: true,
	}
	pos := []float64{1, 2}
	err := fakegantry.MoveToPosition(ctx, pos)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 2")

	pos = []float64{1}
	err = fakegantry.MoveToPosition(ctx, pos)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry position out of range, got 1.00 max is 0.00")

	fakegantry.lengthMm = float64(4)
	fakegantry.positionLimits = []float64{0, 4}
	fakegantry.limitSwitchPins = []string{"1", "2"}
	err = fakegantry.MoveToPosition(ctx, pos)
	test.That(t, err, test.ShouldBeNil)
}

func TestModelFrame(t *testing.T) {
	fakegantry := &oneAxis{
		name:     "test",
		lengthMm: 1.0,
		axes:     []bool{false, false, true},
		model:    nil,
	}

	m := fakegantry.ModelFrame()
	test.That(t, m, test.ShouldNotBeNil)
}

func TestCurrentInputs(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	fakegantry := &oneAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
		lengthMm:        float64(200),
		positionLimits:  []float64{0, 2},
	}

	input, err := fakegantry.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, input[0].Value, test.ShouldEqual, 100)

	fakegantry = &oneAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1"},
		lengthMm:        float64(200),
		positionLimits:  []float64{0, 2},
		limitType:       switchLimitTypeOnePin,
	}

	input, err = fakegantry.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, input[0].Value, test.ShouldEqual, 100)

	// out of bounds position
	fakegantry = &oneAxis{
		motor:          &inject.Motor{PositionFunc: func(ctx context.Context) (float64, error) { return 5, errors.New("nope") }},
		board:          createFakeBoard(),
		limitHigh:      false,
		logger:         logger,
		rpm:            float64(300),
		lengthMm:       float64(200),
		positionLimits: []float64{0, 0.5},
	}

	input, err = fakegantry.CurrentInputs(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, input, test.ShouldBeNil)
}

func TestGoToInputs(t *testing.T) {
	ctx := context.Background()
	inputs := []referenceframe.Input{}
	logger := golog.NewTestLogger(t)

	fakegantry := &oneAxis{
		board:           createFakeBoard(),
		limitSwitchPins: []string{"1", "2"},
		limitHigh:       true,
		motor:           createFakeMotor(),
		lengthMm:        1.0,
		pulleyRMm:       0.1,
		rpm:             10,
		axes:            []bool{true, false, false},
		limitType:       "",
		positionLimits:  []float64{1, 2},
		model:           nil,
		logger:          logger,
	}
	err := fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 0")

	inputs = []referenceframe.Input{{1.0}, {2.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 2")

	inputs = []referenceframe.Input{{-1.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry position out of range, got -1.00 max is 1.00")

	inputs = []referenceframe.Input{{4.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry position out of range, got 4.00 max is 1.00")

	inputs = []referenceframe.Input{{1.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)
}
