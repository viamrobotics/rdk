package singleaxis

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	motorName = "x"
	testGName = "test"
	boardName = "board"
)

var fakeFrame = &referenceframe.LinkConfig{
	Translation: r3.Vector{X: 0, Y: 1.0, Z: 0},
}

var count = 0

func createFakeMotor() motor.Motor {
	return &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{motor.PositionReporting: true}, nil
		},
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return float64(count + 1), nil
		},
		ResetZeroPositionFunc: func(ctx context.Context, offset float64, extra map[string]interface{}) error { return nil },
		GoToFunc:              func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error { return nil },
		GoForFunc:             func(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error { return nil },
		StopFunc:              func(ctx context.Context, extra map[string]interface{}) error { return nil },
		SetPowerFunc:          func(ctx context.Context, powerPct float64, extra map[string]interface{}) error { return nil },
	}
}

func createLimitBoard() board.Board {
	injectGPIOPin := &inject.GPIOPin{
		GetFunc: func(ctx context.Context, extra map[string]interface{}) (bool, error) { return true, nil },
		SetFunc: func(ctx context.Context, high bool, extra map[string]interface{}) error { return nil },
	}
	return &inject.Board{GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) { return injectGPIOPin, nil }}
}

func createFakeBoard() board.Board {
	pinCount := 0
	injectGPIOPin := &inject.GPIOPin{
		GetFunc: func(ctx context.Context, extra map[string]interface{}) (bool, error) {
			pinCount += 1
			if pinCount%2 == 0 {
				return false, nil
			}
			return true, nil
		},
		SetFunc: func(ctx context.Context, high bool, extra map[string]interface{}) error { return nil },
	}
	return &inject.Board{GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) { return injectGPIOPin, nil }}
}

func createFakeDepsForTestNewSingleAxis(t *testing.T) resource.Dependencies {
	t.Helper()
	deps := make(resource.Dependencies)
	deps[board.Named(boardName)] = createFakeBoard()
	deps[motor.Named(motorName)] = createFakeMotor()
	return deps
}

var setTrue = true

func TestValidate(t *testing.T) {
	fakecfg := &Config{}
	deps, err := fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "motor")

	fakecfg.Motor = motorName
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "length_mm")

	fakecfg.LengthMm = 1.0
	fakecfg.MmPerRevolution = 1.0
	fakecfg.LimitSwitchPins = []string{"1"}
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "board")

	fakecfg.Board = boardName
	fakecfg.MmPerRevolution = 1
	fakecfg.LimitPinEnabled = &setTrue
	deps, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldResemble, []string{fakecfg.Motor, fakecfg.Board})
	test.That(t, fakecfg.GantryRPM, test.ShouldEqual, float64(0))
}

func TestNewSingleAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	deps := createFakeDepsForTestNewSingleAxis(t)
	fakecfg := resource.Config{Name: testGName}
	_, err := newSingleAxis(ctx, deps, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *singleaxis.Config but got <nil>")

	deps = createFakeDepsForTestNewSingleAxis(t)
	fakecfg = resource.Config{
		Name:  testGName,
		Frame: fakeFrame,
		ConvertedAttributes: &Config{
			Motor:           motorName,
			LimitSwitchPins: []string{"1", "2"},
			LengthMm:        1.0,
			Board:           boardName,
			LimitPinEnabled: &setTrue,
			GantryRPM:       float64(300),
		},
	}
	fakegantry, err := newSingleAxis(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, ok := fakegantry.(*singleAxis)
	test.That(t, ok, test.ShouldBeTrue)

	deps = createFakeDepsForTestNewSingleAxis(t)
	fakecfg = resource.Config{
		Name:  testGName,
		Frame: fakeFrame,
		ConvertedAttributes: &Config{
			Motor:           motorName,
			LimitSwitchPins: []string{"1"},
			LengthMm:        1.0,
			Board:           boardName,
			LimitPinEnabled: &setTrue,
			MmPerRevolution: 0.1,
			GantryRPM:       float64(300),
		},
	}
	fakegantry, err = newSingleAxis(ctx, deps, fakecfg, logger)
	_, ok = fakegantry.(*singleAxis)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)

	deps = createFakeDepsForTestNewSingleAxis(t)
	fakecfg = resource.Config{
		Name:  testGName,
		Frame: fakeFrame,
		ConvertedAttributes: &Config{
			Motor:           motorName,
			LimitSwitchPins: []string{"1"},
			LengthMm:        1.0,
			Board:           boardName,
			LimitPinEnabled: &setTrue,
			GantryRPM:       float64(300),
		},
	}
	fakegantry, err = newSingleAxis(ctx, deps, fakecfg, logger)
	_, ok = fakegantry.(*singleAxis)
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, err.Error(), test.ShouldContainSubstring, "gantry with one limit switch per axis needs a mm_per_length ratio defined")

	deps = createFakeDepsForTestNewSingleAxis(t)
	fakecfg = resource.Config{
		Name:  testGName,
		Frame: fakeFrame,
		ConvertedAttributes: &Config{
			Motor:           motorName,
			LimitSwitchPins: []string{"1", "2", "3"},
			LengthMm:        1.0,
			Board:           boardName,
			LimitPinEnabled: &setTrue,
		},
	}

	_, err = newSingleAxis(ctx, deps, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid gantry type: need 1, 2 or 0 pins per axis, have 3 pins")

	deps = make(resource.Dependencies)
	_, err = newSingleAxis(ctx, deps, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "missing from dependencies")

	injectMotor := &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
	}
	deps = make(resource.Dependencies)
	deps[motor.Named(motorName)] = injectMotor
	deps[board.Named(boardName)] = createFakeBoard()

	_, err = newSingleAxis(ctx, deps, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid gantry type")

	injectMotor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: false,
			}, nil
		},
	}

	deps = make(resource.Dependencies)
	deps[motor.Named(motorName)] = injectMotor
	deps[board.Named(boardName)] = createFakeBoard()
	_, err = newSingleAxis(ctx, deps, fakecfg, logger)
	expectedErr := motor.NewFeatureUnsupportedError(motor.PositionReporting, motorName)
	test.That(t, err, test.ShouldBeError, expectedErr)
}

func TestReconfigure(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	deps := createFakeDepsForTestNewSingleAxis(t)
	fakecfg := resource.Config{
		Name:  testGName,
		Frame: fakeFrame,
		ConvertedAttributes: &Config{
			Motor:           motorName,
			LimitSwitchPins: []string{"1", "2"},
			LengthMm:        1.0,
			Board:           boardName,
			LimitPinEnabled: &setTrue,
			GantryRPM:       float64(300),
		},
	}
	fakegantry, err := newSingleAxis(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldBeNil)
	g := fakegantry.(*singleAxis)

	deps = createFakeDepsForTestNewSingleAxis(t)
	newconf := resource.Config{
		Name:  testGName,
		Frame: fakeFrame,
		ConvertedAttributes: &Config{
			Motor:           motorName,
			LimitSwitchPins: []string{"1", "3"},
			LengthMm:        5.0,
			Board:           boardName,
			LimitPinEnabled: &setTrue,
			GantryRPM:       float64(400),
			MmPerRevolution: 10,
		},
	}
	err = fakegantry.Reconfigure(ctx, deps, newconf)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, g.limitSwitchPins, test.ShouldResemble, []string{"1", "3"})
	test.That(t, g.lengthMm, test.ShouldEqual, 5.0)
	test.That(t, g.rpm, test.ShouldEqual, float64(400))
	test.That(t, g.mmPerRevolution, test.ShouldEqual, 10)
}

func TestHome(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &singleAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1"},
	}
	err := fakegantry.home(ctx, len(fakegantry.limitSwitchPins))
	test.That(t, err, test.ShouldBeNil)

	goForErr := errors.New("GoFor failed")
	posErr := errors.New("Position fail")
	fakeMotor := &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: false,
			}, nil
		},
		GoForFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return goForErr
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			return nil
		},
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return math.NaN(), posErr
		},
	}
	fakegantry = &singleAxis{
		motor:  fakeMotor,
		logger: logger,
	}
	err = fakegantry.home(ctx, len(fakegantry.limitSwitchPins))
	test.That(t, err, test.ShouldBeError, posErr)

	fakegantry = &singleAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
	}
	err = fakegantry.home(ctx, len(fakegantry.limitSwitchPins))
	test.That(t, err, test.ShouldBeNil)

	fakegantry = &singleAxis{
		motor: fakeMotor,
	}
	err = fakegantry.home(ctx, len(fakegantry.limitSwitchPins))
	test.That(t, err, test.ShouldBeError, posErr)

	fakegantry = &singleAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
	}
	err = fakegantry.home(ctx, len(fakegantry.limitSwitchPins))
	test.That(t, err, test.ShouldBeNil)
}

func TestHomeLimitSwitch(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &singleAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
	}

	err := fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakegantry.positionLimits, test.ShouldResemble, []float64{1, 1})

	getPosErr := errors.New("failed to get position")
	fakegantry.motor = &inject.Motor{
		GoForFunc:    func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error { return nil },
		StopFunc:     func(ctx context.Context, extra map[string]interface{}) error { return nil },
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) { return 0, getPosErr },
	}
	err = fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldBeError, getPosErr)

	fakegantry.motor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
		GoForFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return errors.New("err")
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error { return nil },
	}
	err = fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.motor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
		GoForFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return nil
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error { return errors.New("err") },
	}
	err = fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.motor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
		GoForFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return errors.New("err")
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error { return nil },
	}
	err = fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	injectGPIOPin := &inject.GPIOPin{}
	injectGPIOPin.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return true, errors.New("not supported")
	}
	injectGPIOPinGood := &inject.GPIOPin{}
	injectGPIOPinGood.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return true, nil
	}

	fakegantry.board = &inject.Board{
		GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) {
			return injectGPIOPin, nil
		},
	}
	err = fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.board = &inject.Board{
		GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) {
			if pin == "1" {
				return injectGPIOPinGood, nil
			}
			return injectGPIOPin, nil
		},
	}
	err = fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestHomeLimitSwitch2(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &singleAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1"},
		lengthMm:        float64(1),
		mmPerRevolution: float64(.1),
	}

	err := fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakegantry.positionLimits, test.ShouldResemble, []float64{1, 11})

	getPosErr := errors.New("failed to get position")
	fakegantry.motor = &inject.Motor{
		GoForFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return nil
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			return nil
		},
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return 0, getPosErr
		},
	}
	err = fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldBeError, getPosErr)

	fakegantry.motor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
		GoForFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return errors.New("not supported")
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error { return nil },
	}
	err = fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	injectGPIOPin := &inject.GPIOPin{}
	injectGPIOPin.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return true, errors.New("not supported")
	}

	fakegantry.board = &inject.Board{
		GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) {
			return injectGPIOPin, nil
		},
	}
	err = fakegantry.homeLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestHomeEncoder(t *testing.T) {
	fakegantry := &singleAxis{}

	resetZeroErr := errors.New("failed to set zero")
	injMotor := &inject.Motor{
		GoForFunc:             func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error { return nil },
		StopFunc:              func(ctx context.Context, extra map[string]interface{}) error { return nil },
		ResetZeroPositionFunc: func(ctx context.Context, offset float64, extra map[string]interface{}) error { return resetZeroErr },
	}
	fakegantry.motor = injMotor
	ctx := context.Background()

	getPosErr := errors.New("failed to get position")
	injMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64, extra map[string]interface{}) error { return nil }
	injMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) { return 0, getPosErr }
	err := fakegantry.homeEncoder(ctx)
	test.That(t, err.Error(), test.ShouldContainSubstring, "get position")

	injMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) { return 0, nil }
	err = fakegantry.homeEncoder(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestTestLimit(t *testing.T) {
	ctx := context.Background()
	fakegantry := &singleAxis{
		limitSwitchPins: []string{"1", "2"},
		motor:           createFakeMotor(),
		board:           createLimitBoard(),
		rpm:             float64(300),
		limitHigh:       true,
	}
	pos, err := fakegantry.testLimit(ctx, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, float64(1))
}

func TestLimitHit(t *testing.T) {
	ctx := context.Background()
	fakegantry := &singleAxis{
		limitSwitchPins: []string{"1", "2", "3"},
		board:           createLimitBoard(),
		limitHigh:       true,
	}

	hit, err := fakegantry.limitHit(ctx, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, hit, test.ShouldEqual, true)
}

func TestPosition(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	fakegantry := &singleAxis{
		motor: &inject.Motor{
			PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
				return map[motor.Feature]bool{
					motor.PositionReporting: false,
				}, nil
			},
			PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) { return 1, nil },
		},
		board:           createFakeBoard(),
		positionLimits:  []float64{0, 1},
		positionRange:   1.0,
		limitHigh:       true,
		limitSwitchPins: []string{"1", "2"},
		logger:          logger,
	}
	positions, err := fakegantry.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, positions, test.ShouldResemble, []float64{0})

	fakegantry = &singleAxis{
		motor: &inject.Motor{
			PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
				return nil, errors.New("not supported")
			},
			PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
				return 1, errors.New("not supported")
			},
		},
		board:           createFakeBoard(),
		limitHigh:       true,
		limitSwitchPins: []string{"1", "2"},
		positionLimits:  []float64{0, 1},
		logger:          logger,
	}
	positions, err = fakegantry.Position(ctx, nil)
	test.That(t, positions, test.ShouldResemble, []float64{})
	test.That(t, err, test.ShouldNotBeNil)
}

func TestLengths(t *testing.T) {
	fakegantry := &singleAxis{
		lengthMm: float64(1.0),
	}
	ctx := context.Background()
	fakelengths, err := fakegantry.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.ShouldHaveLength(t, fakelengths, test.ShouldEqual(float64(1.0)))
}

func TestMoveToPosition(t *testing.T) {
	// Skipping because these tests will change soon
	t.Skip()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &singleAxis{
		logger:    logger,
		board:     createFakeBoard(),
		motor:     createFakeMotor(),
		limitHigh: true,
	}
	pos := []float64{1, 2}
	err := fakegantry.MoveToPosition(ctx, pos, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "needs 1 position to move")

	pos = []float64{1}
	err = fakegantry.MoveToPosition(ctx, pos, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "out of range")

	fakegantry.lengthMm = float64(4)
	fakegantry.positionLimits = []float64{0, 4}
	fakegantry.limitSwitchPins = []string{"1", "2"}
	err = fakegantry.MoveToPosition(ctx, pos, nil)
	test.That(t, err, test.ShouldBeNil)

	fakegantry.lengthMm = float64(4)
	fakegantry.positionLimits = []float64{0.01, .01}
	fakegantry.limitSwitchPins = []string{"1", "2"}
	fakegantry.motor = &inject.Motor{StopFunc: func(ctx context.Context, extra map[string]interface{}) error { return errors.New("err") }}
	err = fakegantry.MoveToPosition(ctx, pos, nil)
	test.That(t, err, test.ShouldNotBeNil)

	injectGPIOPin := &inject.GPIOPin{}
	injectGPIOPin.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return true, errors.New("err")
	}
	injectGPIOPinGood := &inject.GPIOPin{}
	injectGPIOPinGood.GetFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return false, nil
	}

	fakegantry.board = &inject.Board{
		GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) {
			return injectGPIOPin, nil
		},
	}

	fakegantry.board = &inject.Board{GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) { return injectGPIOPin, nil }}
	err = fakegantry.MoveToPosition(ctx, pos, nil)
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.board = &inject.Board{GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) { return injectGPIOPinGood, nil }}
	fakegantry.motor = &inject.Motor{
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error { return nil },
		GoToFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return errors.New("err")
		},
	}
	err = fakegantry.MoveToPosition(ctx, pos, nil)
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.motor = &inject.Motor{GoToFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		return nil
	}}
	err = fakegantry.MoveToPosition(ctx, pos, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestModelFrame(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	deps := createFakeDepsForTestNewSingleAxis(t)
	fakecfg := resource.Config{
		Name:  testGName,
		Frame: fakeFrame,
		ConvertedAttributes: &Config{
			Motor:           motorName,
			LimitSwitchPins: []string{"1", "2"},
			LengthMm:        1.0,
			Board:           boardName,
			LimitPinEnabled: &setTrue,
			GantryRPM:       float64(300),
		},
	}
	fakegantry, _ := newSingleAxis(ctx, deps, fakecfg, logger)
	m := fakegantry.ModelFrame()
	test.That(t, m, test.ShouldNotBeNil)
}

func TestStop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	fakegantry := &singleAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
		lengthMm:        float64(200),
		positionLimits:  []float64{0, 2},
	}

	test.That(t, fakegantry.Stop(ctx, nil), test.ShouldBeNil)
}

func TestCurrentInputs(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	fakegantry := &singleAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1", "2"},
		lengthMm:        float64(200),
		positionLimits:  []float64{0, 2},
		positionRange:   2.0,
	}

	input, err := fakegantry.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, input[0].Value, test.ShouldEqual, 100)

	fakegantry = &singleAxis{
		motor:           createFakeMotor(),
		board:           createFakeBoard(),
		limitHigh:       true,
		logger:          logger,
		rpm:             float64(300),
		limitSwitchPins: []string{"1"},
		lengthMm:        float64(200),
		positionLimits:  []float64{0, 2},
		positionRange:   2.0,
	}

	input, err = fakegantry.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, input[0].Value, test.ShouldEqual, 100)

	// out of bounds position
	fakegantry = &singleAxis{
		motor: &inject.Motor{
			PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
				return 5, errors.New("nope")
			},
		},
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

	fakegantry := &singleAxis{
		board:           createFakeBoard(),
		limitSwitchPins: []string{"1", "2"},
		limitHigh:       true,
		motor:           createFakeMotor(),
		lengthMm:        1.0,
		mmPerRevolution: 0.1,
		rpm:             10,
		positionLimits:  []float64{1, 2},
		model:           nil,
		logger:          logger,
	}
	err := fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldContainSubstring, "needs 1 position to move")

	inputs = []referenceframe.Input{{Value: 1.0}, {Value: 2.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldContainSubstring, "needs 1 position to move")

	inputs = []referenceframe.Input{{Value: -1.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldContainSubstring, "out of range")

	inputs = []referenceframe.Input{{Value: 4.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldContainSubstring, "out of range")

	inputs = []referenceframe.Input{{Value: 1.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)
}
