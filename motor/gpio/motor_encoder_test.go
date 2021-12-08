package gpio

import (
	"context"
	"testing"
	"time"

	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robots/fake"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func nowNanosTest() uint64 {
	return uint64(time.Now().UnixNano())
}

func TestMotorEncoder1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	undo := SetRPMSleepDebug(1, false)
	defer undo()

	cfg := motor.Config{TicksPerRotation: 100}
	real := &fake.Motor{}
	interrupt := &board.BasicDigitalInterrupt{}

	motorIfc, err := NewEncodedMotor(config.Component{}, cfg, real, nil, logger)
	test.That(t, err, test.ShouldBeNil)
	motor := motorIfc.(*EncodedMotor)
	defer func() {
		test.That(t, motor.Close(), test.ShouldBeNil)
	}()
	motor.encoder = board.NewSingleEncoder(interrupt, motor)

	// test some basic defaults
	t.Run("encoded motor testing the basics", func(t *testing.T) {
		isOn, err := motor.IsOn(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isOn, test.ShouldBeFalse)
		supported, err := motor.PositionSupported(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, supported, test.ShouldBeTrue)
	})

	// Set and unset regulated motor parameter
	t.Run("encoded motor testing regulation", func(t *testing.T) {
		test.That(t, motor.IsRegulated(), test.ShouldBeFalse)
		motor.SetRegulated(true)
		test.That(t, motor.IsRegulated(), test.ShouldBeTrue)
		motor.SetRegulated(false)
		test.That(t, motor.IsRegulated(), test.ShouldBeFalse)
	})

	// when we go forward things work
	t.Run("encoded motor testing Go", func(t *testing.T) {
		test.That(t, motor.Go(context.Background(), .01), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)
		test.That(t, real.PowerPct(), test.ShouldEqual, .01)
	})

	// Test stop
	t.Run("encoded motor testing Stop", func(t *testing.T) {
		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 0)
	})

	// Test when  we're in the middle of a GoFor and then call Go, don't turn off
	t.Run("encoded motor testing Go interrupt GoFor", func(t *testing.T) {
		test.That(t, motor.GoFor(context.Background(), 1000, 1), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)
		test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, float32(0))

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.PowerPct(), test.ShouldEqual, float32(1))
		})

		// confirm set point was not reached
		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)

		motor.Go(context.Background(), .25)
		test.That(t, interrupt.Ticks(context.Background(), 1000, nowNanosTest()), test.ShouldBeNil) // go far!
	})

	// Test Go to non controlled
	t.Run("encoded motor testing Go (non controlled)", func(t *testing.T) {
		motor.Go(context.Background(), .25)
		test.That(t, interrupt.Ticks(context.Background(), 1000, nowNanosTest()), test.ShouldBeNil) // go far!

		// we should still be moving at the previous force
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.PowerPct(), test.ShouldEqual, float32(.25))
			test.That(t, real.Direction(), test.ShouldEqual, 1)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos, err := motor.Position(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos, test.ShouldEqual, 20.99)
		})

	})

	// Forward GoFor with RPM + and ROT +
	t.Run("encoded motor testing GoFor (REV + | REV +)", func(t *testing.T) {

		// Check time until stop
		test.That(t, motor.GoFor(context.Background(), 1000, 1), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)
		test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, 1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)

		// Check distance and power
		test.That(t, motor.GoFor(context.Background(), 1000, 1), test.ShouldBeNil)
		atStart := motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, 0.5)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
	})

	// Backward GoFor with RPM - and ROT +
	t.Run("encoded motor testing GoFor (REV - | REV +)", func(t *testing.T) {
		test.That(t, motor.GoFor(context.Background(), -1000, 1), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, -1)
		test.That(t, real.PowerPct(), test.ShouldBeLessThan, 0)

		// Check time till stop
		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, -1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)

		// Check power
		test.That(t, motor.GoFor(context.Background(), -1000, 1), test.ShouldBeNil)
		atStart := motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(t, real.PowerPct(), test.ShouldEqual, -1)
		})
		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
	})

	// Backward GoFor with RPM + and ROT -
	t.Run("encoded motor testing GoFor (REV + | REV -)", func(t *testing.T) {
		test.That(t, motor.GoFor(context.Background(), 1000, -1), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, -1)
		test.That(t, real.PowerPct(), test.ShouldBeLessThan, 0)

		// Check time till stop
		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, -1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)

		// Check power
		test.That(t, motor.GoFor(context.Background(), 1000, -1), test.ShouldBeNil)
		atStart := motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(t, real.PowerPct(), test.ShouldEqual, -1)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
	})

	// Forward GoFor with RPM - and ROT -
	t.Run("encoded motor testing GoFor (REV - | REV -)", func(t *testing.T) {

		// Check time until stop
		test.That(t, motor.GoFor(context.Background(), -1000, -1), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)
		test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, 1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)

		// Check distance and power
		test.That(t, motor.GoFor(context.Background(), -1000, -1), test.ShouldBeNil)
		atStart := motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, 0.5)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
	})
}

func TestMotorEncoderHall(t *testing.T) {
	logger := golog.NewTestLogger(t)
	undo := SetRPMSleepDebug(1, false)
	defer undo()

	cfg := motor.Config{TicksPerRotation: 100}
	real := &fake.Motor{}
	encoderA := &board.BasicDigitalInterrupt{}
	encoderB := &board.BasicDigitalInterrupt{}
	encoder := board.NewHallEncoder(encoderA, encoderB)

	motorIfc, err := NewEncodedMotor(config.Component{}, cfg, real, encoder, logger)
	test.That(t, err, test.ShouldBeNil)

	motor := motorIfc.(*EncodedMotor)
	defer func() {
		test.That(t, motor.Close(), test.ShouldBeNil)
	}()

	motor.RPMMonitorStart()
	testutils.WaitForAssertion(t, func(t testing.TB) {
		pos := encoder.RawPosition()
		test.That(t, pos, test.ShouldEqual, 0)
	})

	// Move zero
	t.Run("motor encoder no motion", func(t *testing.T) {
		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // bounce, we should do nothing
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 0)
		})
	})

	// Move forward
	t.Run("motor encoder move forward", func(t *testing.T) {
		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // this should do nothing because it's the initial state
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 0)
		})

		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // we go from state 3 -> 4
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 1)
		})

		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 4 -> 1
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 2)
		})

		test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 1 -> 2
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 3)
		})

		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 2- -> 3
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 4)
		})
	})

	// Move backwards
	t.Run("motor encoder move backward", func(t *testing.T) {
		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 3 -> 2
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 3)
		})

		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 2 -> 1
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 2)
		})

		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 1 -> 4
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 1)
		})

		test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 4 -> 1
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 0)
		})
	})

	// Test forward GoFor checking for stop
	t.Run("motor encoder test GoFor (forward)", func(t *testing.T) {
		undo := SetRPMSleepDebug(1, false)
		defer undo()

		err := motor.GoFor(context.Background(), 100, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)

		err = motor.GoFor(context.Background(), -100, -1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.PowerPct(), test.ShouldEqual, 1.0)
		})

		for x := 0; x < 100; x++ {
			test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldNotEqual, 1)
		})

	})

	// Test backward GoFor checking for stop
	t.Run("motor encoder test GoFor (backwards)", func(t *testing.T) {
		undo := SetRPMSleepDebug(1, false)
		defer undo()

		err := motor.GoFor(context.Background(), 100, -1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, -1)

		err = motor.GoFor(context.Background(), -100, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, -1)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.PowerPct(), test.ShouldEqual, -1.0)
		})

		for x := 0; x < 100; x++ {
			test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldNotEqual, -1)
		})

	})

}

func TestWrapMotorWithEncoder(t *testing.T) {
	logger := golog.NewTestLogger(t)
	real := &fake.Motor{}

	// don't wrap with no encoder
	t.Run("wrap motor no encoder", func(t *testing.T) {
		m, err := WrapMotorWithEncoder(context.Background(), nil, config.Component{Name: "motor1"}, motor.Config{}, real, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m, test.ShouldEqual, real)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	})

	// enforce need TicksPerRotation
	t.Run("wrap motor with encoder no ticksPerRotation", func(t *testing.T) {
		m, err := WrapMotorWithEncoder(context.Background(), nil, config.Component{Name: "motor1"}, motor.Config{Encoder: "a"}, real, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	})

	b, err := fake.NewBoard(context.Background(), config.Component{ConvertedAttributes: &board.Config{}}, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)

	// enforce need encoder
	t.Run("wrap motor with single encoder", func(t *testing.T) {
		m, err := WrapMotorWithEncoder(context.Background(), b, config.Component{Name: "motor1"}, motor.Config{Encoder: "a", TicksPerRotation: 100}, real, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)

		b.Digitals["a"] = &board.BasicDigitalInterrupt{}
		m, err = WrapMotorWithEncoder(context.Background(), b, config.Component{Name: "motor1"}, motor.Config{Encoder: "a", TicksPerRotation: 100}, real, logger)
		test.That(t, err, test.ShouldBeNil)
		_, ok := m.(*EncodedMotor)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	})

	// enforce need encoder b
	t.Run("wrap motor with hall encoder", func(t *testing.T) {
		m, err := WrapMotorWithEncoder(context.Background(), b, config.Component{Name: "motor1"}, motor.Config{Encoder: "a", TicksPerRotation: 100, EncoderB: "b"}, real, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)

		m, err = WrapMotorWithEncoder(context.Background(), b, config.Component{Name: "motor1"}, motor.Config{Encoder: "a", EncoderB: "b", TicksPerRotation: 100}, real, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	})
}
