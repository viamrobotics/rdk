package builtin

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	fakebase "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/baseremotecontrol"
	"go.viam.com/rdk/testutils/inject"
)

func TestBaseRemoteControl(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	deps := make(resource.Dependencies)
	cfg := &Config{
		BaseName:            "baseTest",
		InputControllerName: "inputTest",
		ControlModeName:     "",
	}

	depNames, err := cfg.Validate("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, utils.NewStringSet(depNames...), test.ShouldResemble, utils.NewStringSet("baseTest", "inputTest"))

	fakeController := &inject.InputController{}
	fakeBase := &fakebase.Base{}

	deps[input.Named(cfg.InputControllerName)] = fakeController
	deps[base.Named(cfg.BaseName)] = fakeBase

	fakeController.RegisterControlCallbackFunc = func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
		extra map[string]interface{},
	) error {
		return nil
	}

	// New base_remote_control check
	cfg.ControlModeName = "joystickControl"
	tmpSvc, err := NewBuiltIn(ctx, deps,
		resource.Config{
			Name:                "base_remote_control",
			API:                 baseremotecontrol.API,
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeNil)
	svc, ok := tmpSvc.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)

	cfg.ControlModeName = "triggerSpeedControl"
	tmpSvc1, err := NewBuiltIn(ctx, deps,
		resource.Config{
			Name:                "base_remote_control",
			API:                 baseremotecontrol.API,
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeNil)
	svc1, ok := tmpSvc1.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)

	cfg.ControlModeName = "arrowControl"
	tmpSvc2, err := NewBuiltIn(ctx, deps,
		resource.Config{
			Name:                "base_remote_control",
			API:                 baseremotecontrol.API,
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeNil)
	svc2, ok := tmpSvc2.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)

	cfg.ControlModeName = "buttonControl"
	tmpSvc3, err := NewBuiltIn(ctx, deps,
		resource.Config{
			Name:                "base_remote_control",
			API:                 baseremotecontrol.API,
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeNil)
	svc3, ok := tmpSvc3.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)

	cfg.ControlModeName = "fail"
	tmpSvc4, err := NewBuiltIn(ctx, deps,
		resource.Config{
			Name:                "base_remote_control",
			API:                 baseremotecontrol.API,
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeNil)
	svc4, ok := tmpSvc4.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)

	// Controller import failure
	delete(deps, input.Named(cfg.InputControllerName))
	deps[base.Named(cfg.BaseName)] = fakeBase

	_, err = NewBuiltIn(ctx, deps,
		resource.Config{
			Name:                "base_remote_control",
			API:                 baseremotecontrol.API,
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeError, errors.New("\"rdk:component:input_controller/inputTest\" missing from dependencies"))

	// Base import failure
	deps[input.Named(cfg.InputControllerName)] = fakeController
	delete(deps, base.Named(cfg.BaseName))

	_, err = NewBuiltIn(ctx, deps,
		resource.Config{
			Name:                "base_remote_control",
			API:                 baseremotecontrol.API,
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeError, errors.New("\"rdk:component:base/baseTest\" missing from dependencies"))

	//  Deps exist but are incorrect component
	deps[input.Named(cfg.InputControllerName)] = fakeController
	deps[base.Named(cfg.BaseName)] = fakeController
	_, err = NewBuiltIn(ctx, deps,
		resource.Config{
			Name:                "base_remote_control",
			API:                 baseremotecontrol.API,
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeError,
		errors.New("dependency \"rdk:component:base/baseTest\" should be an implementation of base.Base but it was a *inject.InputController"))

	// Controller event by mode
	t.Run("controller events joystick control mode", func(t *testing.T) {
		i := svc.ControllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.AbsoluteX)
		test.That(t, i[1], test.ShouldEqual, input.AbsoluteY)
	})

	t.Run("controller events trigger speed control mode", func(t *testing.T) {
		i := svc1.ControllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.AbsoluteX)
		test.That(t, i[1], test.ShouldEqual, input.AbsoluteZ)
		test.That(t, i[2], test.ShouldEqual, input.AbsoluteRZ)
	})

	t.Run("controller events arrow control mode", func(t *testing.T) {
		i := svc2.ControllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.AbsoluteHat0X)
		test.That(t, i[1], test.ShouldEqual, input.AbsoluteHat0Y)
	})

	t.Run("controller events button control mode", func(t *testing.T) {
		i := svc3.ControllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.ButtonNorth)
		test.That(t, i[1], test.ShouldEqual, input.ButtonSouth)
		test.That(t, i[2], test.ShouldEqual, input.ButtonEast)
		test.That(t, i[3], test.ShouldEqual, input.ButtonWest)
	})

	t.Run("controller events button no mode", func(t *testing.T) {
		svc4.mu.Lock()
		svc4.controlMode = 8
		svc4.mu.Unlock()
		i := svc4.ControllerInputs()
		test.That(t, len(i), test.ShouldEqual, 0)
	})

	// JoystickControl
	eventX := input.Event{
		Control: input.AbsoluteX,
		Value:   1.0,
	}

	eventY := input.Event{
		Event:   input.PositionChangeAbs,
		Control: input.AbsoluteY,
		Value:   1.0,
	}

	eventHat0X := input.Event{
		Control: input.AbsoluteHat0X,
		Value:   1.0,
	}

	eventHat0Y := input.Event{
		Control: input.AbsoluteHat0Y,
		Value:   1.0,
	}

	t.Run("joy stick control mode for input X", func(t *testing.T) {
		mmPerSec, degsPerSec := oneJoyStickEvent(eventX, 0.5, 0.6)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, 0.5, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, -1.0, .001)
	})

	t.Run("joy stick control mode for input Y", func(t *testing.T) {
		mmPerSec, degsPerSec := oneJoyStickEvent(eventY, 0.5, 0.6)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, -1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.6, .001)
	})

	// TriggerSpeedControl
	eventZ := input.Event{
		Control: input.AbsoluteZ,
		Value:   1.0,
	}

	eventRZ := input.Event{
		Control: input.AbsoluteRZ,
		Value:   1.0,
	}

	t.Run("trigger speed control mode for input X", func(t *testing.T) {
		mmPerSec, degsPerSec := triggerSpeedEvent(eventX, 0.5, 0.6)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, 0.5, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	t.Run("trigger speed control mode for input Z", func(t *testing.T) {
		mmPerSec, degsPerSec := triggerSpeedEvent(eventZ, 0.8, 0.8)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, 0.75, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.8, .001)
	})

	t.Run("trigger speed control mode for input RZ", func(t *testing.T) {
		mmPerSec, degsPerSec := triggerSpeedEvent(eventRZ, 0.8, 0.8)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, 0.85, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.8, .001)
	})

	t.Run("trigger speed control mode for input Y (invalid event)", func(t *testing.T) {
		mmPerSec, degsPerSec := triggerSpeedEvent(eventY, 0.8, 0.8)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, 0.8, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.8, .001)
	})

	// ArrowControl

	arrows := make(map[input.Control]float64)
	arrows[input.AbsoluteHat0X] = 0.0
	arrows[input.AbsoluteHat0Y] = 0.0

	t.Run("arrow control mode for input X", func(t *testing.T) {
		mmPerSec, degsPerSec, _ := arrowEvent(eventHat0X, arrows)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, 0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, -1.0, .001)
	})

	t.Run("arrow control mode for input Y", func(t *testing.T) {
		mmPerSec, degsPerSec, _ := arrowEvent(eventHat0Y, arrows)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, -1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, -1.0, .001)
	})

	// ButtonControl
	buttons := make(map[input.Control]bool)
	buttons[input.ButtonNorth] = false
	buttons[input.ButtonSouth] = false
	buttons[input.ButtonEast] = false
	buttons[input.ButtonWest] = false

	eventButtonNorthPress := input.Event{
		Event:   input.ButtonPress,
		Control: input.ButtonNorth,
	}

	eventButtonSouthPress := input.Event{
		Event:   input.ButtonPress,
		Control: input.ButtonSouth,
	}

	eventButtonNorthRelease := input.Event{
		Event:   input.ButtonRelease,
		Control: input.ButtonNorth,
	}

	t.Run("button control mode for input X and B", func(t *testing.T) {
		mmPerSec, degsPerSec, _ := buttonControlEvent(eventButtonNorthPress, buttons)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)

		mmPerSec, degsPerSec, _ = buttonControlEvent(eventButtonSouthPress, buttons)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, 0.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)

		mmPerSec, degsPerSec, _ = buttonControlEvent(eventButtonNorthRelease, buttons)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, -1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)
	})

	eventButtonEastPress := input.Event{
		Event:   input.ButtonPress,
		Control: input.ButtonEast,
	}

	eventButtonWestPress := input.Event{
		Event:   input.ButtonPress,
		Control: input.ButtonWest,
	}

	eventButtonEastRelease := input.Event{
		Event:   input.ButtonRelease,
		Control: input.ButtonEast,
	}

	t.Run("button control mode for input Y and A", func(t *testing.T) {
		mmPerSec, degsPerSec, _ := buttonControlEvent(eventButtonEastPress, buttons)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, -1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, -1.0, .001)

		mmPerSec, degsPerSec, _ = buttonControlEvent(eventButtonWestPress, buttons)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, -1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)

		mmPerSec, degsPerSec, _ = buttonControlEvent(eventButtonEastRelease, buttons)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, -1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	err = svc.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = svc1.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = svc2.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = svc3.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = svc4.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Close out check
	err = svc.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestLowLevel(t *testing.T) {
	test.That(t, scaleThrottle(.01), test.ShouldAlmostEqual, 0, .001)
	test.That(t, scaleThrottle(-.01), test.ShouldAlmostEqual, 0, .001)

	test.That(t, scaleThrottle(.33), test.ShouldAlmostEqual, 0.4, .001)
	test.That(t, scaleThrottle(.81), test.ShouldAlmostEqual, 0.9, .001)
	test.That(t, scaleThrottle(1.0), test.ShouldAlmostEqual, 1.0, .001)

	test.That(t, scaleThrottle(-.81), test.ShouldAlmostEqual, -0.9, .001)
	test.That(t, scaleThrottle(-1.0), test.ShouldAlmostEqual, -1.0, .001)
}

func TestSimilar(t *testing.T) {
	test.That(t, similar(r3.Vector{}, r3.Vector{}, 1), test.ShouldBeTrue)
	test.That(t, similar(r3.Vector{X: 2}, r3.Vector{}, 1), test.ShouldBeFalse)
	test.That(t, similar(r3.Vector{Y: 2}, r3.Vector{}, 1), test.ShouldBeFalse)
	test.That(t, similar(r3.Vector{Z: 2}, r3.Vector{}, 1), test.ShouldBeFalse)
}
