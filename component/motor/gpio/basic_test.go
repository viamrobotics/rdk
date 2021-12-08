package gpio

import (
	"context"
	"testing"

	"go.viam.com/core/component/motor"
	"go.viam.com/core/robots/fake"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"go.viam.com/test"
)

// Test the A/B/PWM style IO
func TestMotorABPWM(t *testing.T) {
	ctx := context.Background()
	b := &fake.Board{}
	logger := golog.NewTestLogger(t)

	t.Run("motor (A/B/PWM) initialization errors", func(t *testing.T) {
		m, err := NewMotor(b, motor.Config{Pins: map[string]string{"a": "1", "b": "2", "pwm": "3"}, PWMFreq: 4000}, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m.GoFor(ctx, 50, 10), test.ShouldBeError, errors.New("not supported, define maxRPM attribute"))

		test.That(t, m.SetPower(ctx, .45), test.ShouldBeNil)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(114))

		m, err = NewMotor(b, motor.Config{Pins: map[string]string{"a": "1", "b": "2", "pwm": "3"}, MaxPowerPct: 100, PWMFreq: 4000}, logger)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("max_power_pct must be between 0.06 and 1.0"))
	})

	m, err := NewMotor(b, motor.Config{Pins: map[string]string{"a": "1", "b": "2", "pwm": "3"}, MaxRPM: 100, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor (A/B/PWM) Off testing", func(t *testing.T) {
		test.That(t, m.Off(ctx), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(0))

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B/PWM) Go testing", func(t *testing.T) {
		test.That(t, m.Go(ctx, 0.43), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(109))

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.Go(ctx, -0.44), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(112))

		on, err = m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.Go(ctx, 0), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.GPIO["3"], test.ShouldEqual, false)

		on, err = m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B/PWM) GoFor testing", func(t *testing.T) {
		test.That(t, m.GoFor(ctx, 50, 10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(127))

		test.That(t, m.GoFor(ctx, -50, 10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(127))

		test.That(t, m.GoFor(ctx, 50, -10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(127))

		test.That(t, m.GoFor(ctx, -50, -10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(127))
	})

	t.Run("motor (A/B/PWM) Power testing", func(t *testing.T) {
		test.That(t, m.SetPower(ctx, 0.45), test.ShouldBeNil)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(114))
	})

	t.Run("motor (A/B/PWM) Position testing", func(t *testing.T) {
		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		supported, err := m.PositionSupported(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, supported, test.ShouldBeFalse)
	})

	t.Run("motor (A/B/PWM) Set PWM frequency testing", func(t *testing.T) {
		test.That(t, b.PWMFreq["3"], test.ShouldEqual, 4000)
		test.That(t, b.PWMSetFreq(ctx, "3", 8000), test.ShouldBeNil)
		test.That(t, b.PWMFreq["3"], test.ShouldEqual, 8000)
	})

}

// Test the DIR/PWM style IO
func TestMotorDirPWM(t *testing.T) {
	ctx := context.Background()
	b := &fake.Board{}
	logger := golog.NewTestLogger(t)

	t.Run("motor (DIR/PWM) initialization errors", func(t *testing.T) {
		m, err := NewMotor(b, motor.Config{Pins: map[string]string{"dir": "1", "en": "2", "pwm": "3"}, PWMFreq: 4000}, logger)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, m.GoFor(ctx, 50, 10), test.ShouldBeError, errors.New("not supported, define maxRPM attribute"))

		_, err = NewMotor(b, motor.Config{Pins: map[string]string{"dir": "1", "en": "2", "pwm": "3"}, MaxPowerPct: 100, PWMFreq: 4000}, logger)
		test.That(t, err, test.ShouldBeError, errors.New("max_power_pct must be between 0.06 and 1.0"))
	})

	m, err := NewMotor(b, motor.Config{Pins: map[string]string{"dir": "1", "en": "2", "pwm": "3"}, MaxRPM: 100, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor (DIR/PWM) Off testing", func(t *testing.T) {
		test.That(t, m.Off(ctx), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.PWM["1"], test.ShouldEqual, 0)
		test.That(t, b.PWM["2"], test.ShouldEqual, 255)

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (DIR/PWM) Go testing", func(t *testing.T) {
		test.That(t, m.Go(ctx, 0.43), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(109))
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.Go(ctx, -0.44), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(112))
		on, err = m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.Go(ctx, 0), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.GPIO["3"], test.ShouldEqual, false)

		on, err = m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (DIR/PWM) GoFor testing", func(t *testing.T) {
		test.That(t, m.GoFor(ctx, 50, 10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(127))

		test.That(t, m.GoFor(ctx, -50, 10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(127))

		test.That(t, m.GoFor(ctx, 50, -10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(127))

		test.That(t, m.GoFor(ctx, -50, -10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(127))
	})

	t.Run("motor (DIR/PWM) Power testing", func(t *testing.T) {
		test.That(t, m.SetPower(ctx, 0.45), test.ShouldBeNil)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(114))
	})

	t.Run("motor (DIR/PWM) Position testing", func(t *testing.T) {
		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		supported, err := m.PositionSupported(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, supported, test.ShouldBeFalse)
	})

	t.Run("motor (DIR/PWM) Set PWM frequency testing", func(t *testing.T) {
		test.That(t, b.PWMFreq["3"], test.ShouldEqual, 4000)
		test.That(t, b.PWMSetFreq(ctx, "3", 8000), test.ShouldBeNil)
		test.That(t, b.PWMFreq["3"], test.ShouldEqual, 8000)
	})
}

// Test the A/B only style IO
func TestMotorAB(t *testing.T) {
	ctx := context.Background()
	b := &fake.Board{}
	logger := golog.NewTestLogger(t)

	m, err := NewMotor(b, motor.Config{Pins: map[string]string{"a": "1", "b": "2", "en": "3"}, MaxRPM: 100, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor (A/B) On testing", func(t *testing.T) {
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B) Off testing", func(t *testing.T) {
		test.That(t, m.Off(ctx), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.GPIO["3"], test.ShouldEqual, true)
		test.That(t, b.PWM["1"], test.ShouldEqual, 0)
		test.That(t, b.PWM["2"], test.ShouldEqual, 0)

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B) Go testing", func(t *testing.T) {
		test.That(t, m.Go(ctx, 0.43), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.PWM["2"], test.ShouldEqual, byte(145))
		test.That(t, b.GPIO["3"], test.ShouldEqual, false)
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.Go(ctx, -0.44), test.ShouldBeNil)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.PWM["1"], test.ShouldEqual, byte(142))
		test.That(t, b.GPIO["3"], test.ShouldEqual, false)
		on, err = m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.Go(ctx, 0), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.GPIO["3"], test.ShouldEqual, true)

		on, err = m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B) GoFor testing", func(t *testing.T) {
		test.That(t, m.GoFor(ctx, 50, 10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["1"], test.ShouldEqual, byte(255))
		test.That(t, b.PWM["2"], test.ShouldEqual, byte(127))

		test.That(t, m.GoFor(ctx, -50, 10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.PWM["1"], test.ShouldEqual, byte(127))
		test.That(t, b.PWM["2"], test.ShouldEqual, byte(255))

		test.That(t, m.GoFor(ctx, 50, -10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.PWM["1"], test.ShouldEqual, byte(127))
		test.That(t, b.PWM["2"], test.ShouldEqual, byte(255))

		test.That(t, m.GoFor(ctx, -50, -10), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["1"], test.ShouldEqual, byte(255))
		test.That(t, b.PWM["2"], test.ShouldEqual, byte(127))
	})

	t.Run("motor (A/B) Power testing", func(t *testing.T) {
		test.That(t, m.SetPower(ctx, .45), test.ShouldBeNil)
		test.That(t, b.PWM["2"], test.ShouldEqual, byte(140))
	})

	t.Run("motor (A/B) Position testing", func(t *testing.T) {
		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		supported, err := m.PositionSupported(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, supported, test.ShouldBeFalse)
	})

	t.Run("motor (A/B) Set PWM frequency testing", func(t *testing.T) {
		test.That(t, b.PWMFreq["2"], test.ShouldEqual, 4000)
		test.That(t, b.PWMSetFreq(ctx, "2", 8000), test.ShouldBeNil)
		test.That(t, b.PWMFreq["2"], test.ShouldEqual, 8000)
	})
}
