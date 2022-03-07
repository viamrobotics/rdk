package gpio

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	fakeboard "go.viam.com/rdk/component/board/fake"
	"go.viam.com/rdk/component/motor"
)

const maxRPM = 100

// Test the A/B/PWM style IO.
func TestMotorABPWM(t *testing.T) {
	ctx := context.Background()
	b := &fakeboard.Board{}
	logger := golog.NewTestLogger(t)

	t.Run("motor (A/B/PWM) initialization errors", func(t *testing.T) {
		m, err := NewMotor(b, motor.Config{Pins: motor.PinConfig{A: "1", B: "2", PWM: "3"}, MaxPowerPct: 100, PWMFreq: 4000}, logger)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("max_power_pct must be between 0.06 and 1.0"))
	})

	m, err := NewMotor(b, motor.Config{Pins: motor.PinConfig{A: "1", B: "2", PWM: "3"}, MaxRPM: maxRPM, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor (A/B/PWM) Off testing", func(t *testing.T) {
		test.That(t, m.Stop(ctx), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(0))

		on, err := m.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B/PWM) SetPower testing", func(t *testing.T) {
		gpioMotor, ok := m.(*Motor)
		test.That(t, ok, test.ShouldBeTrue)

		test.That(t, gpioMotor.setPWM(ctx, 0.43), test.ShouldBeNil)
		on, err := gpioMotor.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.SetPower(ctx, 0.43), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, true)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(109))

		on, err = m.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.SetPower(ctx, -0.44), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.PWM["3"], test.ShouldEqual, byte(112))

		on, err = m.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.SetPower(ctx, 0), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.GPIO["3"], test.ShouldEqual, false)

		on, err = m.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	//nolint:dupl
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

	t.Run("motor (A/B/PWM) GetPosition testing", func(t *testing.T) {
		pos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		features, err := m.GetFeatures(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor (A/B/PWM) Set PWM frequency testing", func(t *testing.T) {
		test.That(t, b.PWMFreq["3"], test.ShouldEqual, 4000)
		test.That(t, b.SetPWMFreq(ctx, "3", 8000), test.ShouldBeNil)
		test.That(t, b.PWMFreq["3"], test.ShouldEqual, 8000)
	})
}

// Test the DIR/PWM style IO.
func TestMotorDirPWM(t *testing.T) {
	ctx := context.Background()
	b := &fakeboard.Board{}
	logger := golog.NewTestLogger(t)

	t.Run("motor (DIR/PWM) initialization errors", func(t *testing.T) {
		m, err := NewMotor(b, motor.Config{Pins: motor.PinConfig{Dir: "1", En: "2", PWM: "3"}, PWMFreq: 4000}, logger)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, m.GoFor(ctx, 50, 10), test.ShouldBeError, errors.New("not supported, define max_rpm attribute"))

		_, err = NewMotor(
			b,
			motor.Config{Pins: motor.PinConfig{Dir: "1", En: "2", PWM: "3"}, MaxPowerPct: 100, PWMFreq: 4000},
			logger,
		)
		test.That(t, err, test.ShouldBeError, errors.New("max_power_pct must be between 0.06 and 1.0"))
	})

	m, err := NewMotor(b, motor.Config{Pins: motor.PinConfig{Dir: "1", En: "2", PWM: "3"}, MaxRPM: 100, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor (DIR/PWM) Off testing", func(t *testing.T) {
		test.That(t, m.Stop(ctx), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, true)
		test.That(t, b.PWM["1"], test.ShouldEqual, 0)
		test.That(t, b.PWM["2"], test.ShouldEqual, 255)

		on, err := m.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})
	//nolint:dupl
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

		on, err := m.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)
	})

	t.Run("motor (DIR/PWM) GetPosition testing", func(t *testing.T) {
		pos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		features, err := m.GetFeatures(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor (DIR/PWM) Set PWM frequency testing", func(t *testing.T) {
		test.That(t, b.PWMFreq["3"], test.ShouldEqual, 4000)
		test.That(t, b.SetPWMFreq(ctx, "3", 8000), test.ShouldBeNil)
		test.That(t, b.PWMFreq["3"], test.ShouldEqual, 8000)
	})
}

// Test the A/B only style IO.
func TestMotorAB(t *testing.T) {
	ctx := context.Background()
	b := &fakeboard.Board{}
	logger := golog.NewTestLogger(t)

	m, err := NewMotor(b, motor.Config{Pins: motor.PinConfig{A: "1", B: "2", En: "3"}, MaxRPM: maxRPM, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor (A/B) On testing", func(t *testing.T) {
		on, err := m.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B) Off testing", func(t *testing.T) {
		test.That(t, m.Stop(ctx), test.ShouldBeNil)
		test.That(t, b.GPIO["1"], test.ShouldEqual, false)
		test.That(t, b.GPIO["2"], test.ShouldEqual, false)
		test.That(t, b.GPIO["3"], test.ShouldEqual, true)
		test.That(t, b.PWM["1"], test.ShouldEqual, 0)
		test.That(t, b.PWM["2"], test.ShouldEqual, 0)

		on, err := m.IsPowered(ctx)
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

	t.Run("motor (A/B) GetPosition testing", func(t *testing.T) {
		pos, err := m.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		features, err := m.GetFeatures(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor (A/B) Set PWM frequency testing", func(t *testing.T) {
		test.That(t, b.PWMFreq["2"], test.ShouldEqual, 4000)
		test.That(t, b.SetPWMFreq(ctx, "2", 8000), test.ShouldBeNil)
		test.That(t, b.PWMFreq["2"], test.ShouldEqual, 8000)
	})
}

func TestGoForMath(t *testing.T) {
	powerPct, waitDur := goForMath(100, 100, 100)
	test.That(t, powerPct, test.ShouldEqual, 1)
	test.That(t, waitDur, test.ShouldEqual, time.Minute)

	powerPct, waitDur = goForMath(100, -100, 100)
	test.That(t, powerPct, test.ShouldEqual, -1)
	test.That(t, waitDur, test.ShouldEqual, time.Minute)

	powerPct, waitDur = goForMath(100, -1000, 100)
	test.That(t, powerPct, test.ShouldEqual, -1)
	test.That(t, waitDur, test.ShouldEqual, time.Minute)

	powerPct, waitDur = goForMath(100, 1000, 200)
	test.That(t, powerPct, test.ShouldEqual, 1)
	test.That(t, waitDur, test.ShouldEqual, 2*time.Minute)

	powerPct, waitDur = goForMath(100, 1000, 50)
	test.That(t, powerPct, test.ShouldEqual, 1)
	test.That(t, waitDur, test.ShouldEqual, 30*time.Second)
}
