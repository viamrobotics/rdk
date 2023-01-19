package oneaxis

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
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

var (
	counter = 0
)

func createfakemotor() motor.Motor {
	return &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{motor.PositionReporting: true}, nil
		},
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			counter++
			return float64(counter), nil
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
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot find motor for gantry")

	fakecfg.Motor = motorName
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "non-zero and positive")

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
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot find board for gantry")

	fakecfg.Board = "board"
	deps, err = fakecfg.Validate("path")
	test.That(t, deps, test.ShouldBeNil)
	test.That(
		t,
		err.Error(),
		test.ShouldContainSubstring,
		"gantry has one limit switch per axis, needs pulley radius to set position limits",
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

	fakecfg = config.Component{
		Name: testGName,
		ConvertedAttributes: &AttrConfig{
			Motor: motorName,
			Board: "board",
			Axis:  r3.Vector{X: 1, Y: 0, Z: 0},
		},
	}
	fakegantry, err := setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, ok := fakegantry.(*oneAxis)
	test.That(t, ok, test.ShouldBeTrue)

	fakecfg.ConvertedAttributes.(*AttrConfig).LimitPinEnabled = &setTrue
	fakecfg.ConvertedAttributes.(*AttrConfig).LimitSwitchPins = []string{"1", "2"}
	fakegantry, err = setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, ok = fakegantry.(*oneAxis)
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = fakegantry.(*limitSwitchGantry)
	test.That(t, ok, test.ShouldBeTrue)

	fakecfg.ConvertedAttributes.(*AttrConfig).LimitSwitchPins = []string{"1"}
	fakegantry, err = setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldResemble, errDimensionsNotFound(testGName, 0, 0))
	_, ok = fakegantry.(*oneAxis)
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = fakegantry.(*limitSwitchGantry)
	test.That(t, ok, test.ShouldBeFalse)

	fakecfg.ConvertedAttributes.(*AttrConfig).MmPerRevolution = 1
	fakecfg.ConvertedAttributes.(*AttrConfig).LengthMm = 100
	fakegantry, err = setUpGantry(ctx, deps, fakecfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, ok = fakegantry.(*oneAxis)
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = fakegantry.(*limitSwitchGantry)
	test.That(t, ok, test.ShouldBeTrue)

	fakecfg.ConvertedAttributes.(*AttrConfig).LimitSwitchPins = []string{"1", "2", "3"}
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
	fakeOneAx := &oneAxis{
		name:            testGName,
		motor:           createfakemotor(),
		board:           createfakeboard(),
		logger:          logger,
		rpm:             float64(300),
		lengthMm:        100,
		mmPerRevolution: 1,
	}

	fakeLimited := &limitSwitchGantry{
		oAx:             fakeOneAx,
		limitHigh:       true,
		limitSwitchPins: []string{"1", "2"},
	}

	err := fakeOneAx.homeEncoder(ctx)
	test.That(t, err, test.ShouldBeNil)
	lastPos := counter + int(fakeOneAx.lengthMm)/int(fakeOneAx.mmPerRevolution)
	test.That(t, fakeOneAx.positionLimits, test.ShouldResemble, []float64{float64(counter), float64(lastPos)})

	fakeLimited.limitSwitchPins = []string{"1"}
	err = fakeLimited.homeWithLimSwitch(ctx, fakeLimited.limitSwitchPins)
	test.That(t, err, test.ShouldBeNil)
	lastPos = counter + int(fakeLimited.oAx.lengthMm)
	test.That(t, fakeLimited.oAx.positionLimits, test.ShouldResemble, []float64{float64(counter), float64(lastPos)})

	fakeLimited.oAx.lengthMm = 0
	err = fakeLimited.homeWithLimSwitch(ctx, fakeLimited.limitSwitchPins)
	test.That(t, err, test.ShouldResemble, errDimensionsNotFound(testGName,
		fakeLimited.oAx.lengthMm,
		fakeLimited.oAx.mmPerRevolution))

	goForErr := errors.New("GoFor failed")
	posErr := errors.New("Position failed")
	fakeOneAx.lengthMm = 100
	fakeOneAx.motor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: false,
			}, nil
		},
		GoForFunc:    func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error { return goForErr },
		StopFunc:     func(ctx context.Context, extra map[string]interface{}) error { return nil },
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) { return 1.0, posErr },
	}
	fakeLimited.oAx = fakeOneAx
	err = fakeLimited.homeWithLimSwitch(ctx, fakeLimited.limitSwitchPins)
	test.That(t, err, test.ShouldBeError, goForErr)

	err = fakeOneAx.homeEncoder(ctx)
	test.That(t, err, test.ShouldBeError, posErr)
}

func TestTestLimit(t *testing.T) {
	ctx := context.Background()
	baseG := &oneAxis{
		motor: createfakemotor(),
		board: createfakeboard(),
		rpm:   float64(300),
	}

	fakegantry := &limitSwitchGantry{
		limitSwitchPins: []string{"1", "2"},
		oAx:             baseG,
		limitHigh:       true,
	}

	pos, err := fakegantry.testLimit(ctx, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, counter)

	pos, err = fakegantry.testLimit(ctx, 1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, counter)
}

func TestLimitHit(t *testing.T) {
	ctx := context.Background()
	baseG := &oneAxis{
		board: createfakeboard(),
	}

	fakegantry := &limitSwitchGantry{
		limitSwitchPins: []string{"1", "2", "3"},
		limitHigh:       true,
		oAx:             baseG,
	}

	hit, err := fakegantry.limitHit(ctx, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, hit, test.ShouldEqual, true)
}

func TestPosition(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	fakeOAx := &oneAxis{
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
	fakeLim := &limitSwitchGantry{
		oAx: fakeOAx,
	}
	positions, err := fakeOAx.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, positions, test.ShouldResemble, []float64{0})
	positions, err = fakeLim.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, positions, test.ShouldResemble, []float64{0})

	fakeOAx.motor = &inject.Motor{
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return nil, errors.New("not supported")
		},
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return 1, errors.New("not supported")
		},
	}
	fakeLim.oAx = fakeOAx
	positions, err = fakeOAx.Position(ctx, nil)
	test.That(t, positions, test.ShouldResemble, []float64{})
	test.That(t, err, test.ShouldNotBeNil)
	positions, err = fakeLim.Position(ctx, nil)
	test.That(t, positions, test.ShouldResemble, []float64{})
	test.That(t, err, test.ShouldNotBeNil)
}

func TestLengths(t *testing.T) {
	fakeOAx := &oneAxis{
		lengthMm: float64(1.0),
	}
	ctx := context.Background()
	fakelengths, err := fakeOAx.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.ShouldHaveLength(t, fakelengths, test.ShouldEqual(float64(1.0)))

	fakeLim := &limitSwitchGantry{
		oAx: fakeOAx,
	}
	fakelengths, err = fakeLim.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.ShouldHaveLength(t, fakelengths, test.ShouldEqual(float64(1.0)))
}

func TestMoveToPosition(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	baseG := &oneAxis{
		logger: logger,
	}
	fakegantry := &limitSwitchGantry{
		oAx:       baseG,
		limitHigh: true,
	}

	fakegantry.oAx.motor = &inject.Motor{
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error { return nil },
		GoToFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
			return errors.New("err")
		},
	}
	pos := []float64{1, 2}
	err := fakegantry.MoveToPosition(ctx, pos, &referenceframe.WorldState{}, nil)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 2")

	pos = []float64{1}
	err = fakegantry.MoveToPosition(ctx, pos, &referenceframe.WorldState{}, nil)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry position out of range, got 1.00 max is 0.00")

	err = fakegantry.MoveToPosition(ctx, pos, &referenceframe.WorldState{}, nil)
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.oAx.lengthMm = float64(4)
	fakegantry.oAx.positionLimits = []float64{0, 4}
	fakegantry.oAx.motor = &inject.Motor{GoToFunc: func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		return nil
	}}
	err = fakegantry.MoveToPosition(ctx, pos, &referenceframe.WorldState{}, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestModelFrame(t *testing.T) {
	fakeOAx := &oneAxis{
		name:     testGName,
		lengthMm: 1.0,
		model:    referenceframe.NewSimpleModel("gantry"),
	}

	fakeLim := &limitSwitchGantry{
		oAx: fakeOAx,
	}
	m := fakeLim.ModelFrame()
	test.That(t, m, test.ShouldNotBeNil)
}

func TestStop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	fakeOAx := &oneAxis{
		motor:          createfakemotor(),
		board:          createfakeboard(),
		logger:         logger,
		rpm:            float64(300),
		lengthMm:       float64(200),
		positionLimits: []float64{0, 2},
	}

	test.That(t, fakeOAx.Stop(ctx, nil), test.ShouldBeNil)

	fakeLim := &limitSwitchGantry{
		oAx: fakeOAx,
	}
	test.That(t, fakeLim.Stop(ctx, nil), test.ShouldBeNil)
}

func TestCurrentInputs(t *testing.T) {
	ctx := context.Background()

	fakeOAx := &oneAxis{
		lengthMm:       float64(200),
		positionLimits: []float64{0, 2},
		gantryRange:    2,
	}
	fakeOAx.motor = &inject.Motor{
		PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return (fakeOAx.positionLimits[1] - fakeOAx.positionLimits[0]) / fakeOAx.gantryRange, nil
		},
	}
	input, err := fakeOAx.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, input[0].Value, test.ShouldEqual, 100)

	fakeLim := &limitSwitchGantry{
		oAx: fakeOAx,
	}
	input, err = fakeLim.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, input[0].Value, test.ShouldEqual, 100)

	// motor position error
	fakeOAx = &oneAxis{
		motor: &inject.Motor{
			PositionFunc: func(ctx context.Context, extra map[string]interface{}) (float64, error) {
				return 5, errors.New("nope")
			},
		},
		positionLimits: []float64{0, 0.5},
	}

	fakeOAx.gantryRange = fakeOAx.positionLimits[1] - fakeOAx.positionLimits[0]
	input, err = fakeOAx.CurrentInputs(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, input, test.ShouldBeNil)

	fakeLim = &limitSwitchGantry{
		oAx: fakeOAx,
	}
	input, err = fakeLim.CurrentInputs(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, input, test.ShouldBeNil)
}

func TestGoToInputs(t *testing.T) {
	ctx := context.Background()
	inputs := []referenceframe.Input{}

	fakeOAx := &oneAxis{
		motor:          createfakemotor(),
		lengthMm:       1.0,
		positionLimits: []float64{1, 2},
	}
	fakeLim := &limitSwitchGantry{
		oAx: fakeOAx,
	}
	test.That(t, fakeOAx.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 0")
	test.That(t, fakeLim.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 0")

	inputs = []referenceframe.Input{{Value: 1.0}, {Value: 2.0}}
	test.That(t, fakeOAx.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 2")
	test.That(t, fakeLim.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 2")

	inputs = []referenceframe.Input{{Value: -1.0}}
	test.That(t, fakeOAx.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry position out of range, got -1.00 max is 1.00")
	test.That(t, fakeLim.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry position out of range, got -1.00 max is 1.00")

	inputs = []referenceframe.Input{{Value: 4.0}}
	test.That(t, fakeOAx.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry position out of range, got 4.00 max is 1.00")
	test.That(t, fakeLim.GoToInputs(ctx, inputs).Error(),
		test.ShouldEqual, "oneAxis gantry position out of range, got 4.00 max is 1.00")

	inputs = []referenceframe.Input{{Value: 1.0}}
	test.That(t, fakeOAx.GoToInputs(ctx, inputs), test.ShouldBeNil)
	test.That(t, fakeLim.GoToInputs(ctx, inputs), test.ShouldBeNil)
}

func TestIsMoving(t *testing.T) {
	ctx := context.Background()
	fakeOAx := &oneAxis{
		motor:          createfakemotor(),
		lengthMm:       1.0,
		positionLimits: []float64{1, 2},
	}
	fakeLim := &limitSwitchGantry{
		oAx: fakeOAx,
	}

	moving, err := fakeOAx.IsMoving(ctx)
	test.That(t, moving, test.ShouldBeFalse)
	test.That(t, err, test.ShouldBeNil)

	moving, err = fakeLim.IsMoving(ctx)
	test.That(t, moving, test.ShouldBeFalse)
	test.That(t, err, test.ShouldBeNil)
}
