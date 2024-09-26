package gpio

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	encoderName = "encoder"
	motorName   = "motor"
	boardName   = "board"
)

type injectedState struct {
	mu       sync.Mutex
	position float64
	powerPct float64
}

func newState() *injectedState {
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
		return vals.position, encoder.PositionTypeTicks, nil
	}
	enc.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
		return encoder.Properties{TicksCountSupported: true}, nil
	}
	return enc
}

func injectBoard() board.Board {
	gpioPin := inject.GPIOPin{
		SetFunc:    func(ctx context.Context, high bool, extra map[string]interface{}) error { return nil },
		GetFunc:    func(ctx context.Context, extra map[string]interface{}) (bool, error) { return true, nil },
		SetPWMFunc: func(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error { return nil },
	}

	b := inject.NewBoard(boardName)
	b.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
		return &gpioPin, nil
	}

	return b
}

func injectMotor(vals *injectedState) motor.Motor {
	m := inject.NewMotor(motorName)
	m.SetPowerFunc = func(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		vals.mu.Lock()
		defer vals.mu.Unlock()
		vals.powerPct = powerPct
		vals.position += sign(powerPct)
		return nil
	}
	m.GoForFunc = func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		return nil
	}
	m.GoToFunc = func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
		return nil
	}
	m.ResetZeroPositionFunc = func(ctx context.Context, offset float64, extra map[string]interface{}) error {
		return nil
	}
	m.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0.0, nil
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
	m.IsMovingFunc = func(ctx context.Context) (bool, error) {
		on, _, err := m.IsPowered(ctx, nil)
		return on, err
	}
	return m
}

func TestEncodedMotor(t *testing.T) {
	logger := logging.NewTestLogger(t)

	vals := newState()
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
		initpos, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m.GoFor(context.Background(), 10, 1, nil), test.ShouldBeNil)
		finalpos, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, initpos < finalpos, test.ShouldBeTrue)
	})

	t.Run("encoded motor test GoFor backwards", func(t *testing.T) {
		initpos, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m.GoFor(context.Background(), -10, 1, nil), test.ShouldBeNil)
		finalpos, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, initpos > finalpos, test.ShouldBeTrue)
	})

	t.Run("encoded motor test GoFor zero revolutions", func(t *testing.T) {
		test.That(t, m.GoFor(context.Background(), 10, 0, nil), test.ShouldBeError, motor.NewZeroRevsError())
	})

	t.Run("encoded motor test encodedGoForMath", func(t *testing.T) {
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, m.ResetZeroPosition(context.Background(), 0, nil), test.ShouldBeNil)
		})

		// positive rpm and positive revolutions
		expectedGoalPos, expectedGoalRPM, expectedDirection := 4.0, 10.0, 1.0
		goalPos, goalRPM, direction := encodedGoForMath(10, 4, 0, 1)
		test.That(t, goalPos, test.ShouldEqual, expectedGoalPos)
		test.That(t, goalRPM, test.ShouldEqual, expectedGoalRPM)
		test.That(t, direction, test.ShouldEqual, expectedDirection)

		// positive rpm and negative revolutions
		expectedGoalPos, expectedGoalRPM, expectedDirection = -4.0, -10.0, -1.0
		goalPos, goalRPM, direction = encodedGoForMath(10, -4, 0, 1)
		test.That(t, goalPos, test.ShouldEqual, expectedGoalPos)
		test.That(t, goalRPM, test.ShouldEqual, expectedGoalRPM)
		test.That(t, direction, test.ShouldEqual, expectedDirection)

		// negative rpm and positive revolutions
		expectedGoalPos, expectedGoalRPM, expectedDirection = -4.0, -10.0, -1.0
		goalPos, goalRPM, direction = encodedGoForMath(-10, 4, 0, 1)
		test.That(t, goalPos, test.ShouldEqual, expectedGoalPos)
		test.That(t, goalRPM, test.ShouldEqual, expectedGoalRPM)
		test.That(t, direction, test.ShouldEqual, expectedDirection)

		// negative rpm and negative revolutions
		expectedGoalPos, expectedGoalRPM, expectedDirection = 4.0, 10.0, 1.0
		goalPos, goalRPM, direction = encodedGoForMath(-10, -4, 0, 1)
		test.That(t, goalPos, test.ShouldEqual, expectedGoalPos)
		test.That(t, goalRPM, test.ShouldEqual, expectedGoalRPM)
		test.That(t, direction, test.ShouldEqual, expectedDirection)

		// positive rpm and zero revolutions
		expectedGoalPos, expectedGoalRPM, expectedDirection = 0.0, 0.0, 0.0
		goalPos, goalRPM, direction = encodedGoForMath(10, 0, 0, 1)
		test.That(t, goalPos, test.ShouldEqual, expectedGoalPos)
		test.That(t, goalRPM, test.ShouldEqual, expectedGoalRPM)
		test.That(t, direction, test.ShouldEqual, expectedDirection)

		// negative rpm and zero revolutions
		goalPos, goalRPM, direction = encodedGoForMath(-10, 0, 0, 1)
		test.That(t, goalPos, test.ShouldEqual, expectedGoalPos)
		test.That(t, goalRPM, test.ShouldEqual, expectedGoalRPM)
		test.That(t, direction, test.ShouldEqual, expectedDirection)

		// zero rpm and zero revolutions
		goalPos, goalRPM, direction = encodedGoForMath(0, 0, 0, 1)
		test.That(t, goalPos, test.ShouldEqual, expectedGoalPos)
		test.That(t, goalRPM, test.ShouldEqual, expectedGoalRPM)
		test.That(t, direction, test.ShouldEqual, expectedDirection)
	})

	t.Run("encoded motor test SetPower interrupts GoFor", func(t *testing.T) {
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		wg := sync.WaitGroup{}
		wg.Add(1)
		utils.PanicCapturingGo(func() {
			defer wg.Done()
			err := m.GoFor(ctxTimeout, 10, 100, nil) // arbitrarily long blocking call
			test.That(t, err, test.ShouldBeNil)
		})

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			on, powerPct, err := m.IsPowered(context.Background(), nil)
			test.That(tb, on, test.ShouldBeTrue)
			test.That(tb, powerPct, test.ShouldBeGreaterThan, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		err = m.SetPower(context.Background(), -0.5, nil)
		test.That(t, err, test.ShouldBeNil)
		on, powerPct, err := m.IsPowered(context.Background(), nil)
		test.That(t, on, test.ShouldBeTrue)
		test.That(t, powerPct, test.ShouldBeLessThan, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		wg.Wait()
		cancel()
	})
}
