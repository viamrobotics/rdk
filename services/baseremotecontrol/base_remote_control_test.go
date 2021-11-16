package baseremotecontrol

import (
	"context"
	"testing"

	"go.viam.com/core/base"
	"go.viam.com/core/config"
	"go.viam.com/core/input"
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

	svc, err := New(ctx, fakeRobot,
		config.Service{
			Name:                "base_remote_control",
			Type:                "base_remote_control",
			ConvertedAttributes: &Config{},
		},
		rlog.Logger)

	test.That(t, err, test.ShouldBeNil)

	// Starting point: high speed, straight
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
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.1, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.4, .001)
	})

	// Starting point: low speed, arcing
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
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.1, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.4, .001)
	})

}
