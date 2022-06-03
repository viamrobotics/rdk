package oneaxis

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/motor/fake"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func createFakeMotor() *inject.Motor {
	fakeMotor := &inject.Motor{}

	fakeMotor.GetFeaturesFunc = func(ctx context.Context) (map[motor.Feature]bool, error) {
		return map[motor.Feature]bool{
			motor.PositionReporting: true,
		}, nil
	}

	fakeMotor.GetPositionFunc = func(ctx context.Context) (float64, error) { return 1, nil }

	fakeMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64) error { return nil }

	fakeMotor.GoToFunc = func(ctx context.Context, rpm float64, position float64) error { return nil }
	fakeMotor.GoForFunc = func(ctx context.Context, rpm float64, revolutions float64) error { return nil }

	fakeMotor.StopFunc = func(ctx context.Context) error { return nil }

	fakeMotor.SetPowerFunc = func(ctx context.Context, powerPct float64) error { return nil }

	return fakeMotor
}

func createFakeBoard() *inject.Board {
	fakeBoard := &inject.Board{}

	injectGPIOPin := &inject.GPIOPin{}
	fakeBoard.GPIOPinByNameFunc = func(pin string) (board.GPIOPin, error) {
		return injectGPIOPin, nil
	}
	injectGPIOPin.GetFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}
	return fakeBoard
}

func createFakeRobot() *inject.Robot {
	fakerobot := &inject.Robot{}

	fakerobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name.Subtype {
		case board.Subtype:
			injectGPIOPin := &inject.GPIOPin{}
			injectGPIOPin.SetFunc = func(ctx context.Context, high bool) error {
				return nil
			}
			injectGPIOPin.GetFunc = func(ctx context.Context) (bool, error) {
				return true, nil
			}
			return &inject.Board{GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) {
				return injectGPIOPin, nil
			}}, nil
		case motor.Subtype:
			return &fake.Motor{}, nil
		}
		return nil, utils.NewResourceNotFoundError(name)
	}

	return fakerobot
}

var setTrue = true

func TestValidate(t *testing.T) {
	fakecfg := &AttrConfig{}
	err := fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot find motor for gantry")

	fakecfg.Motor = "x"
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "non-zero and positive")

	fakecfg.LengthMm = 1.0
	fakecfg.LimitSwitchPins = []string{}
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "gantry axis undefined")

	fakecfg.Board = "board"
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "assign boards or controllers")

	fakecfg.Board = ""
	fakecfg.LimitSwitchPins = []string{"1"}
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot find board for gantry")

	fakecfg.Board = "board"
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "gantry has one limit switch per axis, needs pulley radius to set position limits")

	fakecfg.LimitSwitchPins = []string{"1", "2"}
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "limit pin enabled")

	fakecfg.LimitPinEnabled = &setTrue
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "gantry axis undefined")

	fakecfg.Axis = spatial.TranslationConfig{X: 1, Y: 1, Z: 0}
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "only one translational")

	fakecfg.Axis = spatial.TranslationConfig{X: 1, Y: 0, Z: 1}
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "only one translational")

	fakecfg.Axis = spatial.TranslationConfig{X: 0, Y: 1, Z: 1}
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "only one translational")

	fakecfg.Axis = spatial.TranslationConfig{X: 1, Y: 1, Z: 1}
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "only one translational")

	fakecfg.Axis = spatial.TranslationConfig{X: 1, Y: 0, Z: 0}
	err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestNewOneAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakeRobot := createFakeRobot()
	fakecfg := config.Component{Name: "gantry"}
	_, err := newOneAxis(ctx, fakeRobot, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *oneaxis.AttrConfig but got <nil>")

	gantryMotorName := "x"

	fakecfg = config.Component{
		Name: "gantry",
		ConvertedAttributes: &AttrConfig{
			Motor:           gantryMotorName,
			LimitSwitchPins: []string{"1", "2"},
			LengthMm:        1.0,
			Board:           "board",
			LimitPinEnabled: &setTrue,
			GantryRPM:       float64(300),
		},
	}
	fakegantry, err := newOneAxis(ctx, fakeRobot, fakecfg, logger)
	fakeoneax, ok := fakegantry.(*oneAxis)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeoneax.limitType, test.ShouldEqual, "twoPin")

	fakecfg = config.Component{
		Name: "gantry",
		ConvertedAttributes: &AttrConfig{
			Motor:           gantryMotorName,
			LimitSwitchPins: []string{"1"},
			LengthMm:        1.0,
			Board:           "board",
			LimitPinEnabled: &setTrue,
			MmPerRevolution: 0.1,
			GantryRPM:       float64(300),
		},
	}
	fakegantry, err = newOneAxis(ctx, fakeRobot, fakecfg, logger)
	fakeoneax, ok = fakegantry.(*oneAxis)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeoneax.limitType, test.ShouldEqual, "onePinOneLength")

	fakecfg = config.Component{
		Name: "gantry",
		ConvertedAttributes: &AttrConfig{
			Motor:           gantryMotorName,
			LimitSwitchPins: []string{"1"},
			LengthMm:        1.0,
			Board:           "board",
			LimitPinEnabled: &setTrue,
			GantryRPM:       float64(300),
		},
	}
	fakegantry, err = newOneAxis(ctx, fakeRobot, fakecfg, logger)
	_, ok = fakegantry.(*oneAxis)
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, err.Error(), test.ShouldContainSubstring, "gantry with one limit switch per axis needs a mm_per_length ratio defined")

	fakecfg = config.Component{
		Name: "gantry",
		ConvertedAttributes: &AttrConfig{
			Motor:           gantryMotorName,
			LimitSwitchPins: []string{"1", "2", "3"},
			LengthMm:        1.0,
			Board:           "board",
		},
	}
	_, err = newOneAxis(ctx, fakeRobot, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid gantry type: need 1, 2 or 0 pins per axis, have 3 pins")

	fakeRobot = &inject.Robot{ResourceByNameFunc: func(name resource.Name) (interface{}, error) {
		return nil, utils.NewResourceNotFoundError(name)
	}}
	_, err = newOneAxis(ctx, fakeRobot, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	injectMotor := &inject.Motor{
		GetFeaturesFunc: func(ctx context.Context) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
	}
	fakeRobot = &inject.Robot{
		ResourceByNameFunc: func(name resource.Name) (interface{}, error) {
			if name.Subtype == motor.Subtype {
				return injectMotor, nil
			}
			return nil, utils.NewResourceNotFoundError(name)
		},
	}

	_, err = newOneAxis(ctx, fakeRobot, fakecfg, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid gantry type")

	injectMotor = &inject.Motor{
		GetFeaturesFunc: func(ctx context.Context) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: false,
			}, nil
		},
	}
	fakeRobot = &inject.Robot{
		ResourceByNameFunc: func(name resource.Name) (interface{}, error) {
			if name.Subtype == motor.Subtype {
				return injectMotor, nil
			}
			return nil, utils.NewResourceNotFoundError(name)
		},
	}
	_, err = newOneAxis(ctx, fakeRobot, fakecfg, logger)
	expectedErr := motor.NewFeatureUnsupportedError(motor.PositionReporting, gantryMotorName)
	test.That(t, err, test.ShouldBeError, expectedErr)
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
		limitSwitchPins: []string{"1"},
		limitType:       "onePinOneLength",
	}
	err := fakegantry.Home(ctx)
	test.That(t, err, test.ShouldBeNil)

	fakeMotor := &inject.Motor{}
	goForErr := errors.New("GoFor failed")
	fakeMotor.GetFeaturesFunc = func(ctx context.Context) (map[motor.Feature]bool, error) {
		return map[motor.Feature]bool{
			motor.PositionReporting: false,
		}, nil
	}
	fakeMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64) error {
		return goForErr
	}
	fakeMotor.StopFunc = func(ctx context.Context) error {
		return nil
	}
	fakegantry = &oneAxis{
		motor:     fakeMotor,
		limitType: "onePinOneLength",
	}
	err = fakegantry.Home(ctx)
	test.That(t, err, test.ShouldBeError, goForErr)

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
		motor:     fakeMotor,
		limitType: "twoPin",
	}
	err = fakegantry.Home(ctx)
	test.That(t, err, test.ShouldBeError, goForErr)

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
	test.That(t, err, test.ShouldBeNil)

	fakegantry.motor = &inject.Motor{
		ResetZeroPositionFunc: func(ctx context.Context, offset float64) error { return nil },
		GetPositionFunc:       func(ctx context.Context) (float64, error) { return 1.0, nil },
	}
	err = fakegantry.Home(ctx)
	test.That(t, err, test.ShouldBeNil)
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

	getPosErr := errors.New("failed to get position")
	fakegantry.motor = &inject.Motor{
		GoForFunc:       func(ctx context.Context, rpm, rotations float64) error { return nil },
		StopFunc:        func(ctx context.Context) error { return nil },
		GetPositionFunc: func(ctx context.Context) (float64, error) { return 0, getPosErr },
	}
	err = fakegantry.homeTwoLimSwitch(ctx)
	test.That(t, err, test.ShouldBeError, getPosErr)

	fakegantry.motor = &inject.Motor{
		GetFeaturesFunc: func(ctx context.Context) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
		GoForFunc: func(ctx context.Context, rpm float64, rotations float64) error { return errors.New("err") },
		StopFunc:  func(ctx context.Context) error { return nil },
	}
	err = fakegantry.homeTwoLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.motor = &inject.Motor{
		GetFeaturesFunc: func(ctx context.Context) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
		GoForFunc: func(ctx context.Context, rpm float64, rotations float64) error { return nil },
		StopFunc:  func(ctx context.Context) error { return errors.New("err") },
	}
	err = fakegantry.homeTwoLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.motor = &inject.Motor{
		GetFeaturesFunc: func(ctx context.Context) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
		GoForFunc: func(ctx context.Context, rpm float64, rotations float64) error { return errors.New("err") },
		StopFunc:  func(ctx context.Context) error { return nil },
	}
	err = fakegantry.homeTwoLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	injectGPIOPin := &inject.GPIOPin{}
	injectGPIOPin.GetFunc = func(ctx context.Context) (bool, error) {
		return true, errors.New("not supported")
	}
	injectGPIOPinGood := &inject.GPIOPin{}
	injectGPIOPinGood.GetFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}

	fakegantry.board = &inject.Board{
		GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) {
			return injectGPIOPin, nil
		},
	}
	err = fakegantry.homeTwoLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.board = &inject.Board{
		GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) {
			if pin == "1" {
				return injectGPIOPinGood, nil
			}
			return injectGPIOPin, nil
		},
	}
	err = fakegantry.homeTwoLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)
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
		mmPerRevolution: float64(.1),
	}

	err := fakegantry.homeOneLimSwitch(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakegantry.positionLimits, test.ShouldResemble, []float64{1, 11})

	getPosErr := errors.New("failed to get position")
	fakegantry.motor = &inject.Motor{
		GoForFunc: func(ctx context.Context, rpm, rotations float64) error {
			return nil
		},
		StopFunc: func(ctx context.Context) error {
			return nil
		},
		GetPositionFunc: func(ctx context.Context) (float64, error) {
			return 0, getPosErr
		},
	}
	err = fakegantry.homeOneLimSwitch(ctx)
	test.That(t, err, test.ShouldBeError, getPosErr)

	fakegantry.motor = &inject.Motor{
		GetFeaturesFunc: func(ctx context.Context) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{
				motor.PositionReporting: true,
			}, nil
		},
		GoForFunc: func(ctx context.Context, rpm float64, rotations float64) error { return errors.New("not supported") },
		StopFunc:  func(ctx context.Context) error { return nil },
	}
	err = fakegantry.homeOneLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	injectGPIOPin := &inject.GPIOPin{}
	injectGPIOPin.GetFunc = func(ctx context.Context) (bool, error) {
		return true, errors.New("not supported")
	}

	fakegantry.board = &inject.Board{
		GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) {
			return injectGPIOPin, nil
		},
	}
	err = fakegantry.homeOneLimSwitch(ctx)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestHomeEncoder(t *testing.T) {
	fakegantry := &oneAxis{
		limitType: limitEncoder,
	}

	resetZeroErr := errors.New("failed to set zero")
	injMotor := &inject.Motor{
		GoForFunc:             func(ctx context.Context, rpm, rotations float64) error { return nil },
		StopFunc:              func(ctx context.Context) error { return nil },
		ResetZeroPositionFunc: func(ctx context.Context, offset float64) error { return resetZeroErr },
	}
	fakegantry.motor = injMotor
	ctx := context.Background()

	getPosErr := errors.New("failed to get position")
	injMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64) error { return nil }
	injMotor.GetPositionFunc = func(ctx context.Context) (float64, error) { return 0, getPosErr }
	err := fakegantry.homeEncoder(ctx)
	test.That(t, err.Error(), test.ShouldContainSubstring, "get position")

	injMotor.GetPositionFunc = func(ctx context.Context) (float64, error) { return 0, nil }
	err = fakegantry.homeEncoder(ctx)
	test.That(t, err, test.ShouldBeNil)
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
}

func TestGetPosition(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	fakegantry := &oneAxis{
		motor: &inject.Motor{
			GetFeaturesFunc: func(ctx context.Context) (map[motor.Feature]bool, error) {
				return map[motor.Feature]bool{
					motor.PositionReporting: false,
				}, nil
			},
			GetPositionFunc: func(ctx context.Context) (float64, error) { return 1, nil },
		},
		board:           createFakeBoard(),
		positionLimits:  []float64{0, 1},
		limitHigh:       true,
		limitSwitchPins: []string{"1", "2"},
		limitType:       limitTwoPin,
		logger:          logger,
	}
	positions, err := fakegantry.GetPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, positions, test.ShouldResemble, []float64{0})

	fakegantry = &oneAxis{
		motor: &inject.Motor{
			GetFeaturesFunc: func(ctx context.Context) (map[motor.Feature]bool, error) {
				return nil, errors.New("not supported")
			},
			GetPositionFunc: func(ctx context.Context) (float64, error) { return 1, errors.New("not supported") },
		},
		board:           createFakeBoard(),
		limitHigh:       true,
		limitSwitchPins: []string{"1", "2"},
		limitType:       limitTwoPin,
		positionLimits:  []float64{0, 1},
		logger:          logger,
	}
	positions, err = fakegantry.GetPosition(ctx)
	test.That(t, positions, test.ShouldResemble, []float64{})
	test.That(t, err, test.ShouldNotBeNil)

	injectGPIOPin := &inject.GPIOPin{}
	injectGPIOPin.GetFunc = func(ctx context.Context) (bool, error) {
		return true, errors.New("not supported")
	}
	injectGPIOPinGood := &inject.GPIOPin{}
	injectGPIOPinGood.GetFunc = func(ctx context.Context) (bool, error) {
		return false, nil
	}
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
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	fakegantry := &oneAxis{
		logger:    logger,
		board:     createFakeBoard(),
		motor:     createFakeMotor(),
		limitHigh: true,
	}
	pos := []float64{1, 2}
	err := fakegantry.MoveToPosition(ctx, pos, &commonpb.WorldState{})
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 2")

	pos = []float64{1}
	err = fakegantry.MoveToPosition(ctx, pos, &commonpb.WorldState{})
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry position out of range, got 1.00 max is 0.00")

	fakegantry.lengthMm = float64(4)
	fakegantry.positionLimits = []float64{0, 4}
	fakegantry.limitSwitchPins = []string{"1", "2"}
	err = fakegantry.MoveToPosition(ctx, pos, &commonpb.WorldState{})
	test.That(t, err, test.ShouldBeNil)

	fakegantry.lengthMm = float64(4)
	fakegantry.positionLimits = []float64{0.01, .01}
	fakegantry.limitSwitchPins = []string{"1", "2"}
	fakegantry.motor = &inject.Motor{StopFunc: func(ctx context.Context) error { return errors.New("err") }}
	err = fakegantry.MoveToPosition(ctx, pos, &commonpb.WorldState{})
	test.That(t, err, test.ShouldNotBeNil)

	injectGPIOPin := &inject.GPIOPin{}
	injectGPIOPin.GetFunc = func(ctx context.Context) (bool, error) {
		return true, errors.New("err")
	}
	injectGPIOPinGood := &inject.GPIOPin{}
	injectGPIOPinGood.GetFunc = func(ctx context.Context) (bool, error) {
		return false, nil
	}

	fakegantry.board = &inject.Board{
		GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) {
			return injectGPIOPin, nil
		},
	}

	fakegantry.board = &inject.Board{GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) { return injectGPIOPin, nil }}
	err = fakegantry.MoveToPosition(ctx, pos, &commonpb.WorldState{})
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.board = &inject.Board{GPIOPinByNameFunc: func(pin string) (board.GPIOPin, error) { return injectGPIOPinGood, nil }}
	fakegantry.motor = &inject.Motor{
		StopFunc: func(ctx context.Context) error { return nil },
		GoToFunc: func(ctx context.Context, rpm float64, rotations float64) error { return errors.New("err") },
	}
	err = fakegantry.MoveToPosition(ctx, pos, &commonpb.WorldState{})
	test.That(t, err, test.ShouldNotBeNil)

	fakegantry.motor = &inject.Motor{GoToFunc: func(ctx context.Context, rpm float64, rotations float64) error { return nil }}
	err = fakegantry.MoveToPosition(ctx, pos, &commonpb.WorldState{})
	test.That(t, err, test.ShouldBeNil)
}

func TestModelFrame(t *testing.T) {
	fakegantry := &oneAxis{
		name:     "test",
		lengthMm: 1.0,
		axis:     r3.Vector{X: 0, Y: 0, Z: 1},
		model:    nil,
	}

	m := fakegantry.ModelFrame()
	test.That(t, m, test.ShouldNotBeNil)
}

func TestStop(t *testing.T) {
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

	test.That(t, fakegantry.Stop(ctx), test.ShouldBeNil)
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
		limitType:       limitOnePin,
	}

	input, err = fakegantry.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, input[0].Value, test.ShouldEqual, 100)

	// out of bounds position
	fakegantry = &oneAxis{
		motor:          &inject.Motor{GetPositionFunc: func(ctx context.Context) (float64, error) { return 5, errors.New("nope") }},
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
		mmPerRevolution: 0.1,
		rpm:             10,
		axis:            r3.Vector{},
		limitType:       "",
		positionLimits:  []float64{1, 2},
		model:           nil,
		logger:          logger,
	}
	err := fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 0")

	inputs = []referenceframe.Input{{Value: 1.0}, {Value: 2.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry MoveToPosition needs 1 position, got: 2")

	inputs = []referenceframe.Input{{Value: -1.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry position out of range, got -1.00 max is 1.00")

	inputs = []referenceframe.Input{{Value: 4.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err.Error(), test.ShouldEqual, "oneAxis gantry position out of range, got 4.00 max is 1.00")

	inputs = []referenceframe.Input{{Value: 1.0}}
	err = fakegantry.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)
}
