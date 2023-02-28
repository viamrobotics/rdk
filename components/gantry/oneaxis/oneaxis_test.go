package oneaxis

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

const (
	motorName = "x"
	testGName = "test"
)

var count = 0

func createfakemotor() motor.Motor {
	return &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{motor.PositionReporting: true}, nil
		},
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return float64(count + 1), nil
		}, ResetZeroPositionFunc: func(ctx context.Context, offset float64, extra map[string]interface{}) error { return nil },
		GoToFunc:     func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error { return nil },
		GoForFunc:    func(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error { return nil },
		StopFunc:     func(ctx context.Context, extra map[string]interface{}) error { return nil },
		SetPowerFunc: func(ctx context.Context, powerPct float64, extra map[string]interface{}) error { return nil },
	}
}

func createfakeboard() board.Board {
	injectGPIOPin := &inject.GPIOPin{
		GetFunc: func(ctx context.Context, extra map[string]interface{}) (bool, error) { return true, nil },
		SetFunc: func(ctx context.Context, high bool, extra map[string]interface{}) error { return nil },
	}
	return &inject.Board{GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) { return injectGPIOPin, nil }}
}

func createFakeDepsForTestNewOneAxis(t *testing.T) registry.Dependencies {
	t.Helper()
	deps := make(registry.Dependencies)
	deps[board.Named("board")] = createfakeboard()
	deps[motor.Named(motorName)] = createfakemotor()
	return deps
}

var setTrue = true

func TestValidate(t *testing.T) {
	fakecfg := &AttrConfig{}
	deps, err := fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "motor")

	fakecfg.Motor = motorName
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "length_mm")

	fakecfg.LengthMm = 1.0
	fakecfg.LimitSwitchPins = []string{}
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "gantry axis undefined")

	fakecfg.Board = "board"
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "assign boards or controllers")

	fakecfg.Board = ""
	fakecfg.LimitSwitchPins = []string{"1"}
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "board")

	fakecfg.Board = "board"
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(
		t,
		err.Error(),
		test.ShouldContainSubstring,
		"gantry has one limit switch",
	)

	fakecfg.LimitSwitchPins = []string{"1", "2"}
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "limit pin enabled")

	fakecfg.LimitPinEnabled = &setTrue
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "gantry axis undefined")

	fakecfg.Axis = r3.Vector{X: 1, Y: 1, Z: 0}
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only one translational")

	fakecfg.Axis = r3.Vector{X: 1, Y: 0, Z: 1}
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only one translational")

	fakecfg.Axis = r3.Vector{X: 0, Y: 1, Z: 1}
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only one translational")

	fakecfg.Axis = r3.Vector{X: 1, Y: 1, Z: 1}
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only one translational")

	fakecfg.Axis = r3.Vector{X: 1, Y: 0, Z: 0}
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldResemble, []string{fakecfg.Motor, fakecfg.Board})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakecfg.GantryRPM, test.ShouldEqual, float64(100))
}

func TestNewGantryTypes(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	deps := createFakeDepsForTestNewOneAxis(t)
	fakecfg := config.Component{}
	_, err := setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *oneaxis.AttrConfig but got <nil>")
	fakeattrcfg := &AttrConfig{
		Motor: motorName,
		Board: "board",
		Axis:  r3.Vector{X: 1, Y: 0, Z: 0},
	}

	fakecfg = config.Component{
		Name:                testGName,
		ConvertedAttributes: fakeattrcfg,
	}
	fake1ax, err := setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, ok := fake1ax.(gantry.LocalGantry)
	test.That(t, ok, test.ShouldBeTrue)

	fakeattrcfg.LimitPinEnabled = &setTrue
	fakeattrcfg.LimitSwitchPins = []string{"1", "2"}
	fake1ax, err = setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldResemble, errZeroLengthGantry)
	_, ok = fake1ax.(gantry.LocalGantry)
	test.That(t, ok, test.ShouldBeFalse)

	fakeattrcfg.LimitSwitchPins = []string{"1"}
	fake1ax, err = setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldResemble, errDimensionsNotFound(testGName, 0, 0))
	_, ok = fake1ax.(gantry.LocalGantry)
	test.That(t, ok, test.ShouldBeFalse)

	fakeattrcfg.MmPerRevolution = 1
	fakeattrcfg.LengthMm = 100
	fake1ax, err = setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, ok = fake1ax.(gantry.LocalGantry)
	test.That(t, ok, test.ShouldBeTrue)

	fakeattrcfg.LimitSwitchPins = []string{"1", "2", "3"}
	_, err = setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldResemble, errBadNumLimitSwitches(testGName, 3))

	deps = make(registry.Dependencies)
	_, err = setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "missing from dependencies")

	injectMotor := &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
	}
	deps = make(registry.Dependencies)
	deps[motor.Named(motorName)] = injectMotor

	_, err = setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldResemble, utils.DependencyNotFoundError("board").Error())

	injectMotor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: false,
			}, nil
		},
	}

	deps = make(registry.Dependencies)
	deps[motor.Named(motorName)] = injectMotor
	_, err = setUpGantry(ctx, deps, fakecfg, logger)
	expectedErr := motor.NewFeatureUnsupportedError(motor.PositionReporting, motorName)
	test.That(t, err, test.ShouldBeError, expectedErr)
}

func TestHome(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fake1ax := &oneAxis{
		name:            testGName,
		motor:           createfakemotor(),
		board:           createfakeboard(),
		logger:          logger,
		rpm:             float64(300),
		lengthMm:        100,
		mmPerRevolution: 1,
	}

	fakepins := &limitPins{
		limitHigh:       true,
		limitSwitchPins: []string{"1", "2"},
	}

	err := homeEncoder(ctx, fake1ax)
	test.That(t, err, test.ShouldBeNil)
	lastPos := count + int(fake1ax.lengthMm)/int(fake1ax.mmPerRevolution) + 1
	test.That(t, fake1ax.positionLimits, test.ShouldResemble, []float64{float64(count) + 1, float64(lastPos)})

	fakepins.limitSwitchPins = []string{"1"}
	err = homeWithLimSwitch(ctx, fakepins, fake1ax)
	test.That(t, err, test.ShouldBeNil)
	lastPos = count + int(fake1ax.lengthMm) + 1
	test.That(t, fake1ax.positionLimits, test.ShouldResemble, []float64{float64(count + 1), float64(lastPos)})

	fake1ax.lengthMm = 0
	err = homeWithLimSwitch(ctx, fakepins, fake1ax)
	test.That(t, err, test.ShouldResemble, errDimensionsNotFound(testGName,
		fake1ax.lengthMm,
		fake1ax.mmPerRevolution))

	goForErr := errors.New("GoFor failed")
	posErr := errors.New("Position failed")
	fake1ax.lengthMm = 100
	fake1ax.motor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: false,
			}, nil
		},
		GoForFunc:    func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error { return goForErr },
		StopFunc:     func(ctx context.Context, extra map[string]interface{}) error { return nil },
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) { return 1.0, posErr },
	}
	err = homeWithLimSwitch(ctx, fakepins, fake1ax)
	test.That(t, err, test.ShouldBeError, goForErr)

	err = homeEncoder(ctx, fake1ax)
	test.That(t, err, test.ShouldBeError, posErr)

}

func TestTestLimit(t *testing.T) {
	ctx := context.Background()
	fake1ax := &oneAxis{
		motor: createfakemotor(),
		board: createfakeboard(),
		rpm:   float64(300),
	}

	fakepins := &limitPins{
		limitSwitchPins: []string{"1", "2"},
		limitHigh:       true,
	}

	pos, err := fakepins.testLimit(ctx, 0, fake1ax)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, count+1) // we called inject motors move position once

	pos, err = fakepins.testLimit(ctx, 1, fake1ax)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, count+1) // same as l295
}

func TestLimitHit(t *testing.T) {
	ctx := context.Background()
	fake1ax := &oneAxis{
		board: createfakeboard(),
	}

	fakepins := &limitPins{
		limitSwitchPins: []string{"1", "2", "3"},
		limitHigh:       true,
	}

	hit, err := fakepins.limitHit(ctx, 0, fake1ax)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, hit, test.ShouldEqual, true)
}

func TestPosition(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	fake1ax := &oneAxis{
		motor: &inject.Motor{
			PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
				return map[motor.Feature]bool{
					motor.PositionReporting: false,
				}, nil
			},
			PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) { return 1, nil },
		},
		positionLimits: []float64{0, 1},
		gantryRange:    1,
		logger:         logger,
	}
	positions, err := fake1ax.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, positions, test.ShouldResemble, []float64{0})

	fake1ax.motor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return nil, errors.New("not supported")
		},
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return 1, errors.New("not supported")
		},
	}
	positions, err = fake1ax.Position(ctx, nil)
	test.That(t, positions, test.ShouldResemble, []float64{})
	test.That(t, err, test.ShouldNotBeNil)
}

func TestLengths(t *testing.T) {
	fake1ax := &oneAxis{
		lengthMm: float64(1.0),
	}
	ctx := context.Background()
	fakelengths, err := fake1ax.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.ShouldHaveLength(t, fakelengths, test.ShouldEqual(float64(1.0)))
}

func TestModelFrame(t *testing.T) {
	fake1ax := &oneAxis{
		name:     testGName,
		lengthMm: 1.0,
		model:    referenceframe.NewSimpleModel("gantry"),
	}

	m := fake1ax.ModelFrame()
	test.That(t, m, test.ShouldNotBeNil)
}

func TestStop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	fake1ax := &oneAxis{
		motor:          createfakemotor(),
		board:          createfakeboard(),
		logger:         logger,
		rpm:            float64(300),
		lengthMm:       float64(200),
		positionLimits: []float64{0, 2},
	}

	test.That(t, fake1ax.Stop(ctx, nil), test.ShouldBeNil)
}

func TestIsMoving(t *testing.T) {
	ctx := context.Background()
	fake1ax := &oneAxis{
		motor:          createfakemotor(),
		lengthMm:       1.0,
		positionLimits: []float64{1, 2},
	}

	moving, err := fake1ax.IsMoving(ctx)
	test.That(t, moving, test.ShouldBeFalse)
	test.That(t, err, test.ShouldBeNil)
}

func TestMoveToPosition(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	fake1ax := &oneAxis{
		logger: logger,
	}

	fake1ax.motor = &inject.Motor{
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error { return nil },
		GoToFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return errors.New("err")
		},
	}
	pos := []float64{1, 2}
	err := fake1ax.MoveToPosition(ctx, pos, &referenceframe.WorldState{}, nil)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 2")

	pos = []float64{1}
	err = fake1ax.MoveToPosition(ctx, pos, &referenceframe.WorldState{}, nil)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry position out of range, got 1.00 max is 0.00")

	err = fake1ax.MoveToPosition(ctx, pos, &referenceframe.WorldState{}, nil)
	test.That(t, err, test.ShouldNotBeNil)

	fake1ax.lengthMm = float64(4)
	fake1ax.positionLimits = []float64{0, 4}
	fake1ax.motor = &inject.Motor{GoToFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		return nil
	}}
	err = fake1ax.MoveToPosition(ctx, pos, &referenceframe.WorldState{}, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestCurrentInputs(t *testing.T) {
	ctx := context.Background()

	fake1ax := &oneAxis{
		lengthMm:       float64(200),
		positionLimits: []float64{0, 2},
		gantryRange:    2,
	}
	fake1ax.motor = &inject.Motor{
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return (fake1ax.positionLimits[1] - fake1ax.positionLimits[0]) / fake1ax.gantryRange, nil
		},
	}
	input, err := fake1ax.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, input[0].Value, test.ShouldEqual, 100)

	// motor position error
	fake1ax = &oneAxis{
		motor: &inject.Motor{
			PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
				return 5, errors.New("nope")
			},
		},
		positionLimits: []float64{0, 0.5},
	}

	fake1ax.gantryRange = fake1ax.positionLimits[1] - fake1ax.positionLimits[0]
	input, err = fake1ax.CurrentInputs(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, input, test.ShouldBeNil)
}

func TestGoToInputs(t *testing.T) {
	ctx := context.Background()
	inputs := []referenceframe.Input{}

	fake1ax := &oneAxis{
		motor:          createfakemotor(),
		lengthMm:       1.0,
		positionLimits: []float64{1, 2},
	}
	test.That(t, fake1ax.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 0")

	inputs = []referenceframe.Input{{Value: 1.0}, {Value: 2.0}}
	test.That(t, fake1ax.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 2")

	inputs = []referenceframe.Input{{Value: -1.0}}
	test.That(t, fake1ax.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry position out of range, got -1.00 max is 1.00")

	inputs = []referenceframe.Input{{Value: 4.0}}
	test.That(t, fake1ax.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry position out of range, got 4.00 max is 1.00")

	inputs = []referenceframe.Input{{Value: 1.0}}
	test.That(t, fake1ax.GoToInputs(ctx, inputs), test.ShouldBeNil)
}
