package baseremotecontrol_test

import (
	"context"
	"testing"

	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	"go.viam.com/core/rlog"
	robotimpl "go.viam.com/core/robot/impl"
	baseremotecontrol "go.viam.com/core/services/remotecontrol"

	"go.viam.com/test"

	// necessary hack because robotimpl is imported in web
	// TODO: remove as part of #253
	_ "go.viam.com/core/services/web"
)

func TestBaseRemoteControl(t *testing.T) {
	ctx := context.Background()
	r, err := robotimpl.New(ctx,
		&config.Config{
			Components: []config.Component{
				{
					Name:                "fr-m",
					Model:               "fake",
					Type:                config.ComponentTypeMotor,
					ConvertedAttributes: &motor.Config{},
				},
				{
					Name:                "fl-m",
					Model:               "fake",
					Type:                config.ComponentTypeMotor,
					ConvertedAttributes: &motor.Config{},
				},
				{
					Name:                "br-m",
					Model:               "fake",
					Type:                config.ComponentTypeMotor,
					ConvertedAttributes: &motor.Config{},
				},
				{
					Name:                "bl-m",
					Model:               "fake",
					Type:                config.ComponentTypeMotor,
					ConvertedAttributes: &motor.Config{},
				},
			},
		},
		rlog.Logger,
	)
	test.That(t, err, test.ShouldBeNil)
	defer test.That(t, r.Close(), test.ShouldBeNil)

	svc, _ := baseremotecontrol.New(ctx, r,
		config.Service{
			Name:                "remote-control",
			Type:                "remote-control",
			ConvertedAttributes: &baseremotecontrol.Config{},
		},
		rlog.Logger)

	// Starting point: above threshold
	t.Run("above_threshold_move_below_threshold", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.SpeedAndAngleMathMag(0.4, 0.0, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.4, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)
	})

	t.Run("above_threshold_move_above_threshold", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.SpeedAndAngleMathMag(1.0, 0.1, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.1, .001)
	})

	t.Run("above_threshold_move_above_mag", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.SpeedAndAngleMathMag(0.1, 1.0, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	t.Run("above_threshold_move_below_mag", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.SpeedAndAngleMathMag(0.1, 0.4, 1.0, 0.0)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.1, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.4, .001)
	})

	// Starting point: below threshold
	t.Run("above_threshold_move_below_threshold", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.SpeedAndAngleMathMag(0.4, 0.0, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.4, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.0, .001)
	})

	t.Run("above_threshold_move_above_threshold", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.SpeedAndAngleMathMag(1.0, 0.1, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 1.0, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.1, .001)
	})

	t.Run("above_threshold_move_above_mag", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.SpeedAndAngleMathMag(0.1, 1.0, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.2, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 1.0, .001)
	})

	t.Run("above_threshold_move_below_mag", func(t *testing.T) {
		millisPerSec, degsPerSec := svc.SpeedAndAngleMathMag(0.1, 0.4, 0.2, 0.2)
		test.That(t, millisPerSec, test.ShouldAlmostEqual, 0.1, .001)
		test.That(t, degsPerSec, test.ShouldAlmostEqual, 0.4, .001)
	})

}
