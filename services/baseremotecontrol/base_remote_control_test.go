package baseremotecontrol

import (
	"context"
	"testing"

	"github.com/pkg/errors"

	"go.viam.com/core/base"
	"go.viam.com/core/component/input"
	"go.viam.com/core/config"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/testutils/inject"

	"go.viam.com/test"
)

func TestBaseRemoteControl(t *testing.T) {
	ctx := context.Background()

	fakeRobot := &inject.Robot{}
	fakeRobot.BaseByNameFunc = func(name string) (base.Base, bool) {
		return &fake.Base{}, true
	}

	fakeController := &inject.InputController{}

	fakeRobot.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
		return fakeController, true
	}

	fakeController.RegisterControlCallbackFunc = func(ctx context.Context, control input.Control, triggers []input.EventType, ctrlFunc input.ControlFunction) error {
		return nil
	}

	cfg := &Config{
		BaseName:            "",
		InputControllerName: "",
		JoyStickModeName:    "",
	}

	// New base_remote_control check
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

	cfg.JoyStickModeName = "triggerSpeedControl"
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

	// Controller import failure
	fakeRobot.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
		return fakeController, false
	}

	_, err = New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeError, errors.Errorf("no input controller named %q", cfg.InputControllerName))

	// Base import failure
	fakeRobot.BaseByNameFunc = func(name string) (base.Base, bool) {
		return &fake.Base{}, false
	}

	_, err = New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeError, errors.Errorf("no base named %q", cfg.BaseName))

	// Start checks
	err = svc.start(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Math tests: Starting point - high speed, straight
	t.Run("full speed to half speed", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.4, 0.0, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.4, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)
	})

	t.Run("full speed to full speed slight angle", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(1.0, 0.1, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.1, .001)
	})

	t.Run("full speed to sharp turn", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.1, 1.0, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	t.Run("full speed to gentle turn", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.1, 0.4, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.4, .001)
	})

	// Math tests: Starting point - low speed, arcing
	t.Run("slow arc to straight", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.4, 0.0, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.4, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)
	})

	t.Run("slow arc to full speed slight angle", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(1.0, 0.1, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.1, .001)
	})

	t.Run("slow arc to sharp turn", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.1, 1.0, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.2, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	t.Run("slow arc to slow turn", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.1, 0.4, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.2, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.4, .001)
	})

	// Controller event by mode
	t.Run("controller events one joy stick control mode", func(t *testing.T) {
		i := svc.controllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.AbsoluteX)
		test.That(t, i[1], test.ShouldEqual, input.AbsoluteY)
	})

	t.Run("controller events one trigger speed control mode", func(t *testing.T) {
		i := svc1.controllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.AbsoluteX)
		test.That(t, i[1], test.ShouldEqual, input.AbsoluteZ)
		test.That(t, i[2], test.ShouldEqual, input.AbsoluteRZ)
	})

	// Event tests
	eventX := input.Event{
		Control: input.AbsoluteX,
		Value:   1.0,
	}

	eventY := input.Event{
		Control: input.AbsoluteY,
		Value:   1.0,
	}

	eventZ := input.Event{
		Control: input.AbsoluteZ,
		Value:   1.0,
	}

	eventRZ := input.Event{
		Control: input.AbsoluteRZ,
		Value:   1.0,
	}

	t.Run("trigger speed control mode for input X", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.triggerSpeedEvent(eventX, 0.5, 0.6)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.5, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	t.Run("trigger speed control mode for input Z", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.triggerSpeedEvent(eventZ, 0.8, 0.8)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.75, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.8, .001)
	})

	t.Run("trigger speed control mode for input RZ", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.triggerSpeedEvent(eventRZ, 0.8, 0.8)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.85, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.8, .001)
	})

	t.Run("one joy stick control mode for input X", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.oneJoyStickEvent(eventX, 0.5, 0.6)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.5, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	t.Run("one joy stick control mode for input Y", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.oneJoyStickEvent(eventY, 0.5, 0.6)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.6, .001)
	})

	t.Run("one joy stick control mode for input Z", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.oneJoyStickEvent(eventZ, 0.5, 0.6)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.5, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.6, .001)
	})

	// Close out check
	err = svc.Close()
	test.That(t, err, test.ShouldBeNil)

	t.Run("one joy stick control mode for input Y", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.triggerSpeedEvent(eventZ, 0.5, 0.6)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.45, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.6, .001)
	})

}
