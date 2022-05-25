package baseremotecontrol

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/base"
	fakebase "go.viam.com/rdk/component/base/fake"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func TestBaseRemoteControl(t *testing.T) {
	ctx := context.Background()

	fakeRobot := &inject.Robot{}
	fakeController := &inject.InputController{}

	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name.Subtype {
		case input.Subtype:
			return fakeController, nil
		case base.Subtype:
			return &fakebase.Base{}, nil
		}
		return nil, rutils.NewResourceNotFoundError(name)
	}

	fakeController.RegisterControlCallbackFunc = func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
	) error {
		return nil
	}

	cfg := &Config{
		BaseName:            "",
		InputControllerName: "",
		ControlModeName:     "",
	}

	// New base_remote_control check
	cfg.ControlModeName = "joystickControl"
	tmpSvc, err := New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	svc, ok := tmpSvc.(*remoteService)
	test.That(t, ok, test.ShouldBeTrue)

	cfg.ControlModeName = "triggerSpeedControl"
	tmpSvc1, err := New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	svc1, ok := tmpSvc1.(*remoteService)
	test.That(t, ok, test.ShouldBeTrue)

	cfg.ControlModeName = "arrowControl"
	tmpSvc2, err := New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	svc2, ok := tmpSvc2.(*remoteService)
	test.That(t, ok, test.ShouldBeTrue)

	cfg.ControlModeName = "buttonControl"
	tmpSvc3, err := New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	svc3, ok := tmpSvc3.(*remoteService)
	test.That(t, ok, test.ShouldBeTrue)

	cfg.ControlModeName = "fail"
	tmpSvc4, err := New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	svc4, ok := tmpSvc4.(*remoteService)
	test.That(t, ok, test.ShouldBeTrue)

	// Controller import failure
	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		if name.Subtype == base.Subtype {
			return &fakebase.Base{}, nil
		}
		return nil, rutils.NewResourceNotFoundError(name)
	}

	_, err = New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:input_controller\" not found"))

	// Base import failure
	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		if name.Subtype == input.Subtype {
			return fakeController, nil
		}
		return nil, rutils.NewResourceNotFoundError(name)
	}

	_, err = New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:base\" not found"))

	// Start checks
	err = svc.start(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Controller event by mode
	t.Run("controller events joystick control mode", func(t *testing.T) {
		i := svc.controllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.AbsoluteX)
		test.That(t, i[1], test.ShouldEqual, input.AbsoluteY)
	})

	t.Run("controller events trigger speed control mode", func(t *testing.T) {
		i := svc1.controllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.AbsoluteX)
		test.That(t, i[1], test.ShouldEqual, input.AbsoluteZ)
		test.That(t, i[2], test.ShouldEqual, input.AbsoluteRZ)
	})

	t.Run("controller events arrow control mode", func(t *testing.T) {
		i := svc2.controllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.AbsoluteHat0X)
		test.That(t, i[1], test.ShouldEqual, input.AbsoluteHat0Y)
	})

	t.Run("controller events button control mode", func(t *testing.T) {
		i := svc3.controllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.ButtonNorth)
		test.That(t, i[1], test.ShouldEqual, input.ButtonSouth)
		test.That(t, i[2], test.ShouldEqual, input.ButtonEast)
		test.That(t, i[3], test.ShouldEqual, input.ButtonWest)
	})

	t.Run("controller events button no mode", func(t *testing.T) {
		svc4.controlMode = 8
		i := svc4.controllerInputs()
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

	t.Run("button control mode for input joystick Y (invalid event)", func(t *testing.T) {
		mmPerSec, degsPerSec, _ := buttonControlEvent(eventY, buttons)
		test.That(t, mmPerSec, test.ShouldAlmostEqual, 0.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)
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
	err = utils.TryClose(context.Background(), svc)
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

func TestParseEvent(t *testing.T) {
	state := throttleState{}

	l, a := parseEvent(droneControl, &state, input.Event{Control: input.AbsoluteX, Value: .5})
	test.That(t, similar(state.linearThrottle, r3.Vector{}, .1), test.ShouldBeTrue)
	test.That(t, similar(state.angularThrottle, r3.Vector{}, .1), test.ShouldBeTrue)

	test.That(t, similar(l, r3.Vector{}, .1), test.ShouldBeTrue)
	test.That(t, similar(a, r3.Vector{Z: -.5}, .1), test.ShouldBeTrue)

	l, a = parseEvent(droneControl, &state, input.Event{Control: input.AbsoluteY, Value: .5})
	test.That(t, similar(l, r3.Vector{Z: -.5}, .1), test.ShouldBeTrue)
	test.That(t, similar(a, r3.Vector{}, .1), test.ShouldBeTrue)

	l, a = parseEvent(droneControl, &state, input.Event{Control: input.AbsoluteRX, Value: .5})
	test.That(t, similar(l, r3.Vector{X: .5}, .1), test.ShouldBeTrue)
	test.That(t, similar(a, r3.Vector{}, .1), test.ShouldBeTrue)

	l, a = parseEvent(droneControl, &state, input.Event{Control: input.AbsoluteRY, Value: .5})
	test.That(t, similar(l, r3.Vector{Y: -.5}, .1), test.ShouldBeTrue)
	test.That(t, similar(a, r3.Vector{}, .1), test.ShouldBeTrue)
}

