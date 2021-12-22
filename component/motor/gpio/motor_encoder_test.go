package gpio

import (
	"context"
	"testing"
	"time"

	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/core/component/board"
	fakeboard "go.viam.com/core/component/board/fake"
	"go.viam.com/core/component/motor"
	fakemotor "go.viam.com/core/component/motor/fake"
	"go.viam.com/core/config"
	"go.viam.com/core/rlog"

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
	real := &fakemotor.Motor{}
	interrupt := &board.BasicDigitalInterrupt{}

	motorIfc, err := NewEncodedMotor(config.Component{}, cfg, real, nil, logger)
	test.That(t, err, test.ShouldBeNil)
	motor := motorIfc.(*EncodedMotor)
	defer func() {
		test.That(t, motor.Close(), test.ShouldBeNil)
	}()
	motor.encoder = board.NewSingleEncoder(interrupt, motor)

	t.Run("encoded motor testing the basics", func(t *testing.T) {
		isOn, err := motor.IsOn(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isOn, test.ShouldBeFalse)
		supported, err := motor.PositionSupported(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, supported, test.ShouldBeTrue)
	})

	t.Run("encoded motor testing regulation", func(t *testing.T) {
		test.That(t, motor.IsRegulated(), test.ShouldBeFalse)
		motor.SetRegulated(true)
		test.That(t, motor.IsRegulated(), test.ShouldBeTrue)
		motor.SetRegulated(false)
		test.That(t, motor.IsRegulated(), test.ShouldBeFalse)
	})

	t.Run("encoded motor testing Go", func(t *testing.T) {
		test.That(t, motor.Go(context.Background(), .01), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)
		test.That(t, real.PowerPct(), test.ShouldEqual, .01)
	})

	t.Run("encoded motor testing Stop", func(t *testing.T) {
		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 0)
	})

	t.Run("encoded motor testing Go interrupt GoFor", func(t *testing.T) {
		test.That(t, motor.GoFor(context.Background(), 1000, 1), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)
		test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, float32(0))

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.PowerPct(), test.ShouldEqual, float32(1))
		})

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, 1)

		motor.Go(context.Background(), .25)
		test.That(t, interrupt.Ticks(context.Background(), 1000, nowNanosTest()), test.ShouldBeNil) // go far!
	})

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

	t.Run("encoded motor testing GoFor (REV + | REV +)", func(t *testing.T) {

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

		test.That(t, motor.GoFor(context.Background(), 1000, 1), test.ShouldBeNil)
		atStart := motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, 0.5)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (REV - | REV +)", func(t *testing.T) {
		test.That(t, motor.GoFor(context.Background(), -1000, 1), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, -1)
		test.That(t, real.PowerPct(), test.ShouldBeLessThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, -1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)

		test.That(t, motor.GoFor(context.Background(), -1000, 1), test.ShouldBeNil)
		atStart := motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(t, real.PowerPct(), test.ShouldEqual, -1)
		})
		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (REV + | REV -)", func(t *testing.T) {
		test.That(t, motor.GoFor(context.Background(), 1000, -1), test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, -1)
		test.That(t, real.PowerPct(), test.ShouldBeLessThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, -1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)

		test.That(t, motor.GoFor(context.Background(), 1000, -1), test.ShouldBeNil)
		atStart := motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(t, real.PowerPct(), test.ShouldEqual, -1)
		})

		test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (REV - | REV -)", func(t *testing.T) {

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

	type testHarness struct {
		Encoder   *board.HallEncoder
		EncoderA  board.DigitalInterrupt
		EncoderB  board.DigitalInterrupt
		RealMotor *fakemotor.Motor
		Motor     motor.Motor
		Teardown  func()
	}
	setup := func(t *testing.T) testHarness {
		cfg := motor.Config{TicksPerRotation: 100}
		real := &fakemotor.Motor{}
		encoderA := &board.BasicDigitalInterrupt{}
		encoderB := &board.BasicDigitalInterrupt{}
		encoder := board.NewHallEncoder(encoderA, encoderB)

		motorIfc, err := NewEncodedMotor(config.Component{}, cfg, real, encoder, logger)
		test.That(t, err, test.ShouldBeNil)

		motor := motorIfc.(*EncodedMotor)

		motor.RPMMonitorStart()
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 0)
		})

		return testHarness{
			encoder,
			encoderA,
			encoderB,
			real,
			motor,
			func() { test.That(t, motor.Close(), test.ShouldBeNil) },
		}
	}

	t.Run("motor encoder no motion", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		encoder := th.Encoder
		encoderB := th.EncoderB

		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // bounce, we should do nothing
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 0)
		})
	})

	t.Run("motor encoder move forward", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		encoder := th.Encoder
		encoderA := th.EncoderA
		encoderB := th.EncoderB

		// this should do nothing because it's the initial state
		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 0)
		})

		// we go from state 3 -> 4
		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 1)
		})

		// 4 -> 1
		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 2)
		})

		// 1 -> 2
		test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 3)
		})

		// 2 -> 3
		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 4)
		})
	})

	t.Run("motor encoder move backward", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		encoder := th.Encoder
		encoderA := th.EncoderA
		encoderB := th.EncoderB

		// this should do nothing because it's the initial state
		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 0)
		})

		// we go from state 3 -> 4
		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 1)
		})

		// 4 -> 1
		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 2)
		})

		// 1 -> 2
		test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 3)
		})

		// 2 -> 3
		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 4)
		})

		// 3 -> 2
		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 3)
		})

		// 2 -> 1
		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 2)
		})

		// 1 -> 4
		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 1)
		})

		// 4 -> 1
		test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(t testing.TB) {
			pos := encoder.RawPosition()
			test.That(t, pos, test.ShouldEqual, 0)
		})
	})

	t.Run("motor encoder test GoFor (forward)", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()
		undo := SetRPMSleepDebug(1, false)
		defer undo()

		encoderA := th.EncoderA
		encoderB := th.EncoderB
		realMotor := th.RealMotor
		motor := th.Motor

		err := motor.GoFor(context.Background(), 100, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, realMotor.Direction(), test.ShouldEqual, 1)

		err = motor.GoFor(context.Background(), -100, -1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, realMotor.Direction(), test.ShouldEqual, 1)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, realMotor.PowerPct(), test.ShouldEqual, 1.0)
		})

		for x := 0; x < 100; x++ {
			test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, realMotor.Direction(), test.ShouldNotEqual, 1)
		})

	})

	t.Run("motor encoder test GoFor (backwards)", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()
		undo := SetRPMSleepDebug(1, false)
		defer undo()

		encoderA := th.EncoderA
		encoderB := th.EncoderB
		realMotor := th.RealMotor
		motor := th.Motor

		err := motor.GoFor(context.Background(), 100, -1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, realMotor.Direction(), test.ShouldEqual, -1)

		err = motor.GoFor(context.Background(), -100, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, realMotor.Direction(), test.ShouldEqual, -1)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, realMotor.PowerPct(), test.ShouldEqual, -1.0)
		})

		for x := 0; x < 100; x++ {
			test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, realMotor.Direction(), test.ShouldNotEqual, -1)
		})

	})

}

func TestWrapMotorWithEncoder(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("wrap motor no encoder", func(t *testing.T) {
		real := &fakemotor.Motor{}
		m, err := WrapMotorWithEncoder(
			context.Background(),
			nil,
			config.Component{Name: "motor1"}, motor.Config{},
			real,
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m, test.ShouldEqual, real)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	})

	t.Run("wrap motor with encoder no ticksPerRotation", func(t *testing.T) {
		real := &fakemotor.Motor{}
		m, err := WrapMotorWithEncoder(
			context.Background(),
			nil,
			config.Component{Name: "motor1"}, motor.Config{Encoder: "a"},
			real,
			logger,
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	})

	t.Run("wrap motor with single encoder", func(t *testing.T) {
		b, err := fakeboard.NewBoard(context.Background(), config.Component{ConvertedAttributes: &board.Config{}}, rlog.Logger)
		test.That(t, err, test.ShouldBeNil)
		real := &fakemotor.Motor{}
		m, err := WrapMotorWithEncoder(
			context.Background(),
			b,
			config.Component{Name: "motor1"}, motor.Config{Encoder: "a", TicksPerRotation: 100},
			real,
			logger,
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)

		b.Digitals["a"] = &board.BasicDigitalInterrupt{}
		m, err = WrapMotorWithEncoder(
			context.Background(),
			b,
			config.Component{Name: "motor1"}, motor.Config{Encoder: "a", TicksPerRotation: 100},
			real,
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		_, ok := m.(*EncodedMotor)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	})

	t.Run("wrap motor with hall encoder", func(t *testing.T) {
		b, err := fakeboard.NewBoard(context.Background(), config.Component{ConvertedAttributes: &board.Config{}}, rlog.Logger)
		test.That(t, err, test.ShouldBeNil)
		real := &fakemotor.Motor{}
		m, err := WrapMotorWithEncoder(
			context.Background(),
			b,
			config.Component{Name: "motor1"}, motor.Config{Encoder: "a", TicksPerRotation: 100, EncoderB: "b"},
			real,
			logger,
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)

		m, err = WrapMotorWithEncoder(
			context.Background(),
			b,
			config.Component{Name: "motor1"}, motor.Config{Encoder: "a", EncoderB: "b", TicksPerRotation: 100},
			real,
			logger,
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, m, test.ShouldBeNil)
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	})
}
