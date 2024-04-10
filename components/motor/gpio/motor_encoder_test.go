package gpio

import (
	"context"
	"sync"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	encoderName = "encoder"
	motorName   = "motor"
)

type injectedState struct {
	mu       sync.Mutex
	position float64
	powerPct float64
	enc      encoder.Encoder
	m        motor.Motor
}

func newInjectedState() *injectedState {
	return &injectedState{
		position: 0.0,
		powerPct: 0.0,
	}
}

func injectEncoder(vals *injectedState) encoder.Encoder {
	enc := inject.NewEncoder(encoderName)
	enc.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		vals.mu.Lock()
		defer vals.mu.Unlock()
		vals.position = 0.0
		return nil
	}
	enc.PositionFunc = func(ctx context.Context,
		positionType encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		vals.mu.Lock()
		defer vals.mu.Unlock()
		// fmt.Println("encoder vals", vals.position)
		return vals.position, encoder.PositionTypeTicks, nil
	}
	enc.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
		return encoder.Properties{TicksCountSupported: true}, nil
	}
	return enc
}

func injectMotor(vals *injectedState) motor.Motor {
	m := inject.NewMotor(motorName)
	m.SetPowerFunc = func(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
		vals.mu.Lock()
		defer vals.mu.Unlock()
		vals.powerPct = powerPct
		// fmt.Println("setting power to   ", vals.powerPct)
		vals.position++
		// fmt.Println("getting position also   ", vals.powerPct)
		return nil
	}
	m.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
		return motor.Properties{}, nil
	}
	m.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		vals.mu.Lock()
		defer vals.mu.Unlock()
		vals.powerPct = 0
		return nil
	}
	m.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
		vals.mu.Lock()
		defer vals.mu.Unlock()
		return vals.powerPct != 0, vals.powerPct, nil
	}
	m.IsMovingFunc = func(context.Context) (bool, error) {
		return vals.powerPct != 0, nil
	}
	return m
}

func TestEncodedMotor(t *testing.T) {
	logger := logging.NewTestLogger(t)

	vals := newInjectedState()
	// create inject motor
	fakeMotor := injectMotor(vals)

	// create an inject encoder
	enc := injectEncoder(vals)

	// create an encoded motor
	conf := resource.Config{
		Name:                motorName,
		ConvertedAttributes: &Config{},
	}
	motorConf := Config{
		TicksPerRotation: 1,
	}
	wrappedMotor, err := WrapMotorWithEncoder(context.Background(), enc, conf, motorConf, fakeMotor, logger)
	test.That(t, err, test.ShouldBeNil)
	m, ok := wrappedMotor.(*EncodedMotor)
	test.That(t, ok, test.ShouldBeTrue)

	defer func() {
		test.That(t, m.Close(context.Background()), test.ShouldBeNil)
	}()

	t.Run("encoded motor test Properties and IsMoving", func(t *testing.T) {
		props, err := m.Properties(context.Background(), nil)
		test.That(t, props.PositionReporting, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

		move, err := m.IsMoving(context.Background())
		test.That(t, move, test.ShouldBeFalse)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("encoded motor test SetPower, IsPowered, Stop, Position, and ResetZeroPosition", func(t *testing.T) {
		// set power
		test.That(t, m.SetPower(context.Background(), 0.5, nil), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(tb, on, test.ShouldBeTrue)
			test.That(tb, powerPct, test.ShouldBeGreaterThan, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		// stop motor
		test.That(t, m.Stop(context.Background(), nil), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(tb, on, test.ShouldBeFalse)
			test.That(tb, powerPct, test.ShouldEqual, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		// check that position is positive
		pos, err := m.Position(context.Background(), nil)
		test.That(t, pos, test.ShouldBeGreaterThan, 0)
		test.That(t, err, test.ShouldBeNil)

		// reset position
		test.That(t, m.ResetZeroPosition(context.Background(), 0, nil), test.ShouldBeNil)

		// check that position is now 0
		pos, err = m.Position(context.Background(), nil)
		test.That(t, pos, test.ShouldEqual, 0)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("encoded motor test GoFor forward", func(t *testing.T) {
		go func() {
			test.That(t, m.GoFor(context.Background(), 10, 1, nil), test.ShouldBeNil)
		}()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(tb, on, test.ShouldBeTrue)
			test.That(tb, powerPct, test.ShouldBeGreaterThan, 0)
			test.That(tb, err, test.ShouldBeNil)
		})
	})

	t.Run("encoded motor test GoFor backwards", func(t *testing.T) {
		go func() {
			test.That(t, m.GoFor(context.Background(), 10, 1, nil), test.ShouldBeNil)
		}()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(tb, on, test.ShouldBeTrue)
			test.That(tb, powerPct, test.ShouldBeLessThan, 0)
			test.That(tb, err, test.ShouldBeNil)
		})
	})

	t.Run("encoded motor test SetPower interrupts GoFor", func(t *testing.T) {
		test.That(t, m.ResetZeroPosition(context.Background(), 0, nil), test.ShouldBeNil)
		go func() {
			test.That(t, m.GoFor(context.Background(), 10, 1, nil), test.ShouldBeNil)
		}()

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(tb, on, test.ShouldBeTrue)
			test.That(tb, powerPct, test.ShouldBeGreaterThan, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, m.SetPower(context.Background(), -0.5, nil), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(tb, on, test.ShouldBeTrue)
			test.That(tb, powerPct, test.ShouldBeLessThan, 0)
			test.That(tb, err, test.ShouldBeNil)
		})
	})
}

func TestEncodedMotorControls(t *testing.T) {
	logger := logging.NewTestLogger(t)

	vals := newInjectedState()
	// create inject motor
	fakeMotor := injectMotor(vals)

	// create an inject encoder
	enc := injectEncoder(vals)

	// create an encoded motor
	conf := resource.Config{
		Name:                motorName,
		ConvertedAttributes: &Config{},
	}
	motorConf := Config{
		TicksPerRotation: 1,
		ControlParameters: &motorPIDConfig{
			P: 1,
			I: 2,
			D: 0,
		},
	}
	wrappedMotor, err := WrapMotorWithEncoder(context.Background(), enc, conf, motorConf, fakeMotor, logger)
	test.That(t, err, test.ShouldBeNil)
	m, ok := wrappedMotor.(*EncodedMotor)
	test.That(t, ok, test.ShouldBeTrue)

	defer func() {
		test.That(t, m.Close(context.Background()), test.ShouldBeNil)
	}()

	t.Run("encoded motor controls test loop exists", func(t *testing.T) {
		test.That(t, m.goForInternal(context.Background(), 10, 1, 1), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, m.loop, test.ShouldNotBeNil)
		})
	})
}

func TestMathHelpers(t *testing.T) {
	t.Run("encoded motor test goForMath", func(t *testing.T) {
		expectedGoalPos, expectedGoalRPM, expectedDirection := 4.0, 10.0, 1.0
		goalPos, goalRPM, direction := goForMathEncoded(10, 4, 0, 1)

		test.That(t, goalPos, test.ShouldEqual, expectedGoalPos)
		test.That(t, goalRPM, test.ShouldEqual, expectedGoalRPM)
		test.That(t, direction, test.ShouldEqual, expectedDirection)

		expectedGoalPos, expectedGoalRPM, expectedDirection = -4.0, -10.0, -1.0
		goalPos, goalRPM, direction = goForMathEncoded(10, -4, 0, 1)

		test.That(t, goalPos, test.ShouldEqual, expectedGoalPos)
		test.That(t, goalRPM, test.ShouldEqual, expectedGoalRPM)
		test.That(t, direction, test.ShouldEqual, expectedDirection)
	})

	t.Run("calculate currentRPM", func(t *testing.T) {
		zeroSec := int64(0)
		sixtySec := int64(6e10)
		oneSec := int64(1)
		// ticks per rotation
		tpr := 1.0
		// last time equals current time, set rpm to zero
		currentRpm := calculateCurrentRpm(10, 0, tpr, oneSec, oneSec)
		test.That(t, currentRpm, test.ShouldEqual, 0.)

		// forwards
		currentRpm = calculateCurrentRpm(10, 0, tpr, sixtySec, zeroSec)
		test.That(t, currentRpm, test.ShouldEqual, 10.0)

		// backwards
		currentRpm = calculateCurrentRpm(-10, 0, tpr, sixtySec, zeroSec)
		test.That(t, currentRpm, test.ShouldEqual, -10.0)
	})

	t.Run("calculateNewPower", func(t *testing.T) {
		rampRate := 1.0

		newPower := calculateNewPower(1, 10, 10, 1, rampRate)
		test.That(t, newPower, test.ShouldEqual, 11)

		newPower = calculateNewPower(-10, 10, 1, 1, rampRate)
		test.That(t, newPower, test.ShouldEqual, 2)

		newPower = calculateNewPower(-10, -10, 1, -1, rampRate)
		test.That(t, newPower, test.ShouldEqual, 1)

		newPower = calculateNewPower(10, -10, 1, -1, rampRate)
		test.That(t, newPower, test.ShouldEqual, 1)
	})
}
