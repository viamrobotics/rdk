package gpio

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	fakeboard "go.viam.com/rdk/component/board/fake"
	"go.viam.com/rdk/component/motor"
)

const maxRPM = 100

// Test the A/B/PWM style IO.
func TestMotorABPWM(t *testing.T) {
	ctx := context.Background()
	b := &fakeboard.Board{GPIOPins: map[string]*fakeboard.GPIOPin{}}
	logger := golog.NewTestLogger(t)

	t.Run("motor (A/B/PWM) initialization errors", func(t *testing.T) {
		m, err := NewMotor(b, motor.Config{
			Pins: motor.PinConfig{A: "1", B: "2", PWM: "3"}, MaxPowerPct: 100, PWMFreq: 4000,
		}, logger)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("max_power_pct must be between 0.06 and 1.0"))
	})

	m, err := NewMotor(b, motor.Config{
		Pins:   motor.PinConfig{A: "1", B: "2", PWM: "3"},
		MaxRPM: maxRPM, PWMFreq: 4000,
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor (A/B/PWM) Off testing", func(t *testing.T) {
		test.That(t, m.Stop(ctx, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "3").PWM(context.Background()), test.ShouldEqual, byte(0))

		on, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B/PWM) SetPower testing", func(t *testing.T) {
		gpioMotor, ok := m.(*Motor)
		test.That(t, ok, test.ShouldBeTrue)

		test.That(t, gpioMotor.setPWM(ctx, 0.43), test.ShouldBeNil)
		on, err := gpioMotor.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.SetPower(ctx, 0.43, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "3").PWM(context.Background()), test.ShouldEqual, .43)

		on, err = m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.SetPower(ctx, -0.44, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "3").PWM(context.Background()), test.ShouldEqual, .44)

		on, err = m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		test.That(t, m.SetPower(ctx, 0, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "3").Get(context.Background()), test.ShouldEqual, false)

		on, err = m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B/PWM) GoFor testing", func(t *testing.T) {
		test.That(t, m.GoFor(ctx, 50, 0, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "3").PWM(context.Background()), test.ShouldEqual, .5)

		test.That(t, m.GoFor(ctx, -50, 0, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "3").PWM(context.Background()), test.ShouldEqual, .5)

		test.That(t, m.Stop(ctx, nil), test.ShouldBeNil)
	})

	t.Run("motor (A/B/PWM) Power testing", func(t *testing.T) {
		test.That(t, m.SetPower(ctx, 0.45, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "3").PWM(context.Background()), test.ShouldEqual, .45)
	})

	t.Run("motor (A/B/PWM) GetPosition testing", func(t *testing.T) {
		pos, err := m.GetPosition(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		features, err := m.GetFeatures(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor (A/B/PWM) Set PWM frequency testing", func(t *testing.T) {
		test.That(t, mustGetGPIOPinByName(b, "3").PWMFreq(context.Background()), test.ShouldEqual, 4000)
		mustGetGPIOPinByName(b, "3").SetPWMFreq(ctx, 8000)
		test.That(t, mustGetGPIOPinByName(b, "3").PWMFreq(context.Background()), test.ShouldEqual, 8000)
	})
}

// Test the DIR/PWM style IO.
func TestMotorDirPWM(t *testing.T) {
	ctx := context.Background()
	b := &fakeboard.Board{GPIOPins: map[string]*fakeboard.GPIOPin{}}
	logger := golog.NewTestLogger(t)

	t.Run("motor (DIR/PWM) initialization errors", func(t *testing.T) {
		m, err := NewMotor(b, motor.Config{Pins: motor.PinConfig{Direction: "1", EnablePinLow: "2", PWM: "3"}, PWMFreq: 4000}, logger)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, m.GoFor(ctx, 50, 10, nil), test.ShouldBeError, errors.New("not supported, define max_rpm attribute != 0"))

		_, err = NewMotor(
			b,
			motor.Config{Pins: motor.PinConfig{Direction: "1", EnablePinLow: "2", PWM: "3"}, MaxPowerPct: 100, PWMFreq: 4000},
			logger,
		)
		test.That(t, err, test.ShouldBeError, errors.New("max_power_pct must be between 0.06 and 1.0"))
	})

	m, err := NewMotor(b, motor.Config{
		Pins:   motor.PinConfig{Direction: "1", EnablePinLow: "2", PWM: "3"},
		MaxRPM: maxRPM, PWMFreq: 4000,
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor (DIR/PWM) Off testing", func(t *testing.T) {
		test.That(t, m.Stop(ctx, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "1").PWM(context.Background()), test.ShouldEqual, 0)
		test.That(t, mustGetGPIOPinByName(b, "2").PWM(context.Background()), test.ShouldEqual, 0)

		on, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (DIR/PWM) GoFor testing", func(t *testing.T) {
		test.That(t, m.GoFor(ctx, 50, 0, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "3").PWM(context.Background()), test.ShouldEqual, .5)

		test.That(t, m.GoFor(ctx, -50, 0, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "3").PWM(context.Background()), test.ShouldEqual, .5)

		test.That(t, m.Stop(ctx, nil), test.ShouldBeNil)
	})

	t.Run("motor (DIR/PWM) Power testing", func(t *testing.T) {
		test.That(t, m.SetPower(ctx, 0.45, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "3").PWM(context.Background()), test.ShouldEqual, .45)

		on, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)
	})

	t.Run("motor (DIR/PWM) GetPosition testing", func(t *testing.T) {
		pos, err := m.GetPosition(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		features, err := m.GetFeatures(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor (DIR/PWM) Set PWM frequency testing", func(t *testing.T) {
		test.That(t, mustGetGPIOPinByName(b, "3").PWMFreq(context.Background()), test.ShouldEqual, 4000)
		mustGetGPIOPinByName(b, "3").SetPWMFreq(ctx, 8000)
		test.That(t, mustGetGPIOPinByName(b, "3").PWMFreq(context.Background()), test.ShouldEqual, 8000)
	})
}

// Test the A/B only style IO.
func TestMotorAB(t *testing.T) {
	ctx := context.Background()
	b := &fakeboard.Board{GPIOPins: map[string]*fakeboard.GPIOPin{}}
	logger := golog.NewTestLogger(t)

	m, err := NewMotor(b, motor.Config{
		Pins:   motor.PinConfig{A: "1", B: "2", EnablePinLow: "3"},
		MaxRPM: maxRPM, PWMFreq: 4000,
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor (A/B) On testing", func(t *testing.T) {
		on, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B) Off testing", func(t *testing.T) {
		test.That(t, m.Stop(ctx, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "3").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "1").PWM(context.Background()), test.ShouldEqual, 0)
		test.That(t, mustGetGPIOPinByName(b, "2").PWM(context.Background()), test.ShouldEqual, 0)

		on, err := m.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)
	})

	t.Run("motor (A/B) GoFor testing", func(t *testing.T) {
		test.That(t, m.SetPower(ctx, .5, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "1").PWM(context.Background()), test.ShouldEqual, 0)
		test.That(t, mustGetGPIOPinByName(b, "2").PWM(context.Background()), test.ShouldEqual, .5)

		test.That(t, m.SetPower(ctx, -.5, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "1").Get(context.Background()), test.ShouldEqual, false)
		test.That(t, mustGetGPIOPinByName(b, "2").Get(context.Background()), test.ShouldEqual, true)
		test.That(t, mustGetGPIOPinByName(b, "1").PWM(context.Background()), test.ShouldEqual, .5)
		test.That(t, mustGetGPIOPinByName(b, "2").PWM(context.Background()), test.ShouldEqual, .5)

		test.That(t, m.Stop(ctx, nil), test.ShouldBeNil)
	})

	t.Run("motor (A/B) Power testing", func(t *testing.T) {
		test.That(t, m.SetPower(ctx, .45, nil), test.ShouldBeNil)
		test.That(t, mustGetGPIOPinByName(b, "2").PWM(context.Background()), test.ShouldEqual, .55)
	})

	t.Run("motor (A/B) GetPosition testing", func(t *testing.T) {
		pos, err := m.GetPosition(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		features, err := m.GetFeatures(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeFalse)
	})

	t.Run("motor (A/B) Set PWM frequency testing", func(t *testing.T) {
		test.That(t, mustGetGPIOPinByName(b, "2").PWMFreq(context.Background()), test.ShouldEqual, 4000)
		mustGetGPIOPinByName(b, "2").SetPWMFreq(ctx, 8000)
		test.That(t, mustGetGPIOPinByName(b, "2").PWMFreq(context.Background()), test.ShouldEqual, 8000)
	})
}

func TestMotorABNoEncoder(t *testing.T) {
	ctx := context.Background()
	b := &fakeboard.Board{GPIOPins: map[string]*fakeboard.GPIOPin{}}
	logger := golog.NewTestLogger(t)

	m, err := NewMotor(b, motor.Config{
		Pins:    motor.PinConfig{A: "1", B: "2", EnablePinLow: "3"},
		PWMFreq: 4000,
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor no encoder GoFor testing", func(t *testing.T) {
		test.That(t, m.GoFor(ctx, 50, 10, nil), test.ShouldBeError, errors.New("not supported, define max_rpm attribute != 0"))
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

	powerPct, waitDur = goForMath(200, 100, 50)
	test.That(t, powerPct, test.ShouldAlmostEqual, 0.5)
	test.That(t, waitDur, test.ShouldEqual, 30*time.Second)

	powerPct, waitDur = goForMath(200, 100, -50)
	test.That(t, powerPct, test.ShouldAlmostEqual, -0.5)
	test.That(t, waitDur, test.ShouldEqual, 30*time.Second)

	powerPct, waitDur = goForMath(200, 50, 0)
	test.That(t, powerPct, test.ShouldEqual, 0.25)
	test.That(t, waitDur, test.ShouldEqual, 0)

	powerPct, waitDur = goForMath(200, -50, 0)
	test.That(t, powerPct, test.ShouldEqual, -0.25)
	test.That(t, waitDur, test.ShouldEqual, 0)
}

func mustGetGPIOPinByName(b board.Board, name string) mustGPIOPin {
	pin, err := b.GPIOPinByName(name)
	if err != nil {
		panic(err)
	}
	return mustGPIOPin{pin}
}

type mustGPIOPin struct {
	pin board.GPIOPin
}

func (m mustGPIOPin) Set(ctx context.Context, high bool) {
	if err := m.pin.Set(ctx, high); err != nil {
		panic(err)
	}
}

func (m mustGPIOPin) Get(ctx context.Context) bool {
	ret, err := m.pin.Get(ctx)
	if err != nil {
		panic(err)
	}
	return ret
}

func (m mustGPIOPin) PWM(ctx context.Context) float64 {
	ret, err := m.pin.PWM(ctx)
	if err != nil {
		panic(err)
	}
	return ret
}

func (m mustGPIOPin) SetPWM(ctx context.Context, dutyCyclePct float64) {
	if err := m.pin.SetPWM(ctx, dutyCyclePct); err != nil {
		panic(err)
	}
}

func (m mustGPIOPin) PWMFreq(ctx context.Context) uint {
	ret, err := m.pin.PWMFreq(ctx)
	if err != nil {
		panic(err)
	}
	return ret
}

func (m mustGPIOPin) SetPWMFreq(ctx context.Context, freqHz uint) {
	err := m.pin.SetPWMFreq(ctx, freqHz)
	if err != nil {
		panic(err)
	}
}
