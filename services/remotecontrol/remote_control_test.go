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

	fakeRobot.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
		return &fake.InputController{}, true
	}

	svc, _ := New(ctx, fakeRobot,
		config.Service{
			Name:                "base-remote-control",
			Type:                "base-remote-control",
			ConvertedAttributes: &Config{},
		},
		rlog.Logger)

	_ = svc.Start(ctx)

	// Starting point: above threshold
	t.Run("above_threshold_move_below_threshold", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.4, 0.0, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.4, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)
	})

	t.Run("above_threshold_move_above_threshold", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(1.0, 0.1, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.1, .001)
	})

	t.Run("above_threshold_move_above_mag", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.1, 1.0, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	t.Run("above_threshold_move_below_mag", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.1, 0.4, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.1, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.4, .001)
	})

	// Starting point: below threshold
	t.Run("above_threshold_move_below_threshold", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.4, 0.0, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.4, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)
	})

	t.Run("above_threshold_move_above_threshold", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(1.0, 0.1, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.1, .001)
	})

	t.Run("above_threshold_move_above_mag", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.1, 1.0, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.2, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	t.Run("above_threshold_move_below_mag", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.speedAndAngleMathMag(0.1, 0.4, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.1, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.4, .001)
	})

}
