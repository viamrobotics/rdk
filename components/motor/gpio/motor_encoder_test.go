package gpio

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/motor"
	fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
)

func nowNanosTest() uint64 {
	return uint64(time.Now().UnixNano())
}

type fakeDirectionAware struct {
	m *fakemotor.Motor
}

func (f *fakeDirectionAware) DirectionMoving() int64 {
	return int64(f.m.Direction())
}

func TestMotorEncoder1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	undo := SetRPMSleepDebug(1, false)
	defer undo()

	cfg := Config{TicksPerRotation: 100, MaxRPM: 100}
	fakeMotor := &fakemotor.Motor{
		MaxRPM:           100,
		Logger:           logger,
		TicksPerRotation: 100,
	}
	interrupt := &board.BasicDigitalInterrupt{}

	e := &encoder.SingleEncoder{I: interrupt, CancelCtx: context.Background()}
	e.AttachDirectionalAwareness(&fakeDirectionAware{m: fakeMotor})
	e.Start(context.Background())
	dirFMotor, err := NewEncodedMotor(config.Component{}, cfg, fakeMotor, e, logger)
	test.That(t, err, test.ShouldBeNil)
	_motor, ok := dirFMotor.(*EncodedMotor)
	test.That(t, ok, test.ShouldBeTrue)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), _motor), test.ShouldBeNil)
	}()

	t.Run("encoded motor testing the basics", func(t *testing.T) {
		isOn, powerPct, err := _motor.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isOn, test.ShouldBeFalse)
		test.That(t, powerPct, test.ShouldEqual, 0.0)
		features, err := _motor.Properties(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
	})

	t.Run("encoded motor testing regulation", func(t *testing.T) {
		test.That(t, _motor.IsRegulated(), test.ShouldBeFalse)
		_motor.SetRegulated(true)
		test.That(t, _motor.IsRegulated(), test.ShouldBeTrue)
		_motor.SetRegulated(false)
		test.That(t, _motor.IsRegulated(), test.ShouldBeFalse)
	})

	t.Run("encoded motor testing SetPower", func(t *testing.T) {
		test.That(t, _motor.SetPower(context.Background(), .01, nil), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldEqual, .01)
	})

	t.Run("encoded motor testing Stop", func(t *testing.T) {
		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 0)
	})

	t.Run("encoded motor cannot go at 0 RPM", func(t *testing.T) {
		test.That(t, _motor.GoFor(context.Background(), 0, 1, nil), test.ShouldBeError, motor.NewZeroRPMError())
	})

	t.Run("encoded motor testing SetPower interrupt GoFor", func(t *testing.T) {
		test.That(t, _motor.goForInternal(context.Background(), 1000, 1), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldBeGreaterThan, float32(0))

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.PowerPct(), test.ShouldEqual, float32(1))
		})

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 1)

		_motor.SetPower(context.Background(), .25, nil)
		test.That(t, interrupt.Ticks(context.Background(), 1000, nowNanosTest()), test.ShouldBeNil) // go far!
	})

	t.Run("encoded motor testing Go (non controlled)", func(t *testing.T) {
		_motor.SetPower(context.Background(), .25, nil)
		test.That(t, interrupt.Ticks(context.Background(), 1000, nowNanosTest()), test.ShouldBeNil) // go far!

		// we should still be moving at the previous force
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.PowerPct(), test.ShouldEqual, float32(.25))
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 1)
		})

		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos, err := _motor.Position(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, math.Abs(pos-20.99), test.ShouldBeLessThan, 0.01)
		})
	})

	t.Run("encoded motor testing GoFor (REV + | REV +)", func(t *testing.T) {
		test.That(t, _motor.goForInternal(context.Background(), 1000, 1), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldBeGreaterThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)

		test.That(t, _motor.goForInternal(context.Background(), 1000, 1), test.ShouldBeNil)
		atStart := _motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, _motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(tb, fakeMotor.PowerPct(), test.ShouldBeGreaterThan, 0.5)
		})

		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (REV - | REV +)", func(t *testing.T) {
		test.That(t, _motor.goForInternal(context.Background(), -1000, 1), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, -1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldBeLessThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, -1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)

		test.That(t, _motor.goForInternal(context.Background(), -1000, 1), test.ShouldBeNil)
		atStart := _motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, _motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(tb, fakeMotor.PowerPct(), test.ShouldEqual, -1)
		})
		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (REV + | REV -)", func(t *testing.T) {
		test.That(t, _motor.goForInternal(context.Background(), 1000, -1), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, -1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldBeLessThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, -1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)

		test.That(t, _motor.goForInternal(context.Background(), 1000, -1), test.ShouldBeNil)
		atStart := _motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, _motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(tb, fakeMotor.PowerPct(), test.ShouldEqual, -1)
		})

		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (REV - | REV -)", func(t *testing.T) {
		test.That(t, _motor.goForInternal(context.Background(), -1000, -1), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldBeGreaterThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 1)
		})

		test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 0)
		})

		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)

		test.That(t, _motor.goForInternal(context.Background(), -1000, -1), test.ShouldBeNil)
		atStart := _motor.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, _motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(tb, fakeMotor.PowerPct(), test.ShouldBeGreaterThan, 0.5)
		})

		test.That(t, _motor.Stop(context.Background(), nil), test.ShouldBeNil)
	})

	t.Run("Ensure stop called when gofor is interrupted", func(t *testing.T) {
		ctx := context.Background()
		var wg sync.WaitGroup
		ctx, cancel := context.WithCancel(ctx)
		wg.Add(1)
		go func() {
			_motor.GoFor(ctx, 100, 100, map[string]interface{}{})
			wg.Done()
		}()
		cancel()
		wg.Wait()

		test.That(t, ctx.Err(), test.ShouldNotBeNil)
		test.That(t, _motor.state.desiredRPM, test.ShouldEqual, 0)
	})
}

func TestMotorEncoderIncremental(t *testing.T) {
	logger := golog.NewTestLogger(t)
	undo := SetRPMSleepDebug(1, false)
	defer undo()

	type testHarness struct {
		Encoder   *encoder.IncrementalEncoder
		EncoderA  board.DigitalInterrupt
		EncoderB  board.DigitalInterrupt
		RealMotor *fakemotor.Motor
		Motor     *EncodedMotor
		Teardown  func()
	}
	setup := func(t *testing.T) testHarness {
		t.Helper()
		cfg := Config{TicksPerRotation: 100, MaxRPM: 100}
		fakeMotor := &fakemotor.Motor{
			MaxRPM:           100,
			Logger:           logger,
			TicksPerRotation: 100,
		}
		encoderA := &board.BasicDigitalInterrupt{}
		encoderB := &board.BasicDigitalInterrupt{}
		encoder := &encoder.IncrementalEncoder{A: encoderA, B: encoderB, CancelCtx: context.Background()}
		encoder.Start(context.Background())

		motorIfc, err := NewEncodedMotor(config.Component{}, cfg, fakeMotor, encoder, logger)
		test.That(t, err, test.ShouldBeNil)

		motor, ok := motorIfc.(*EncodedMotor)
		test.That(t, ok, test.ShouldBeTrue)

		motor.RPMMonitorStart()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0)
		})

		return testHarness{
			encoder,
			encoderA,
			encoderB,
			fakeMotor,
			motor,
			func() { test.That(t, utils.TryClose(context.Background(), motor), test.ShouldBeNil) },
		}
	}

	t.Run("motor encoder no motion", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		encoder := th.Encoder
		encoderB := th.EncoderB

		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // bounce, we should do nothing
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0)
		})
	})

	t.Run("motor encoder move forward", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		encoder := th.Encoder
		encoderA := th.EncoderA
		encoderB := th.EncoderB

		// this should do nothing because it's the initial state
		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0)
		})
		test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // we go from state 00 -> 10
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 1)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 10 -> 11
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 2)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 11 -> 01
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 3)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 01 -> 00
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 4)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 2)
			test.That(tb, err, test.ShouldBeNil)
		})
	})

	t.Run("motor encoder move backward", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		encoder := th.Encoder
		encoderA := th.EncoderA
		encoderB := th.EncoderB

		// this should do nothing because it's the initial state
		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0)
		})
		test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // we go from state 00 -> 10
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 1)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 10 -> 11
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 2)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 11 -> 01
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 3)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 01 -> 00
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 4)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 2)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // we go from state 00 -> 01
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 3)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 01 -> 11
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 2)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 11 -> 10
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 1)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 10 -> 00
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := encoder.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0)
			posFl, err := encoder.TicksCount(context.Background(), nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 0)
			test.That(tb, err, test.ShouldBeNil)
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

		err := motor.goForInternal(context.Background(), 100, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, realMotor.Direction(), test.ShouldEqual, 1)

		err = motor.goForInternal(context.Background(), -100, -1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, realMotor.Direction(), test.ShouldEqual, 1)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, realMotor.PowerPct(), test.ShouldEqual, 1.0)
		})

		for x := 0; x < 100; x++ {
			test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, realMotor.Direction(), test.ShouldNotEqual, 1)
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

		err := motor.goForInternal(context.Background(), 100, -1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, realMotor.Direction(), test.ShouldEqual, -1)

		err = motor.goForInternal(context.Background(), -100, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, realMotor.Direction(), test.ShouldEqual, -1)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, realMotor.PowerPct(), test.ShouldEqual, -1.0)
		})

		for x := 0; x < 100; x++ {
			test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, realMotor.Direction(), test.ShouldNotEqual, -1)
		})
	})
}

func TestWrapMotorWithEncoder(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("wrap motor no encoder", func(t *testing.T) {
		fakeMotor := &fakemotor.Motor{}
		m, err := WrapMotorWithEncoder(
			context.Background(),
			nil,
			config.Component{Name: "motor1"}, Config{},
			fakeMotor,
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m, test.ShouldEqual, fakeMotor)
		test.That(t, utils.TryClose(context.Background(), m), test.ShouldBeNil)
	})

	t.Run("wrap motor with single encoder", func(t *testing.T) {
		b, err := fakeboard.NewBoard(context.Background(), config.Component{ConvertedAttributes: &fakeboard.Config{}}, logger)
		test.That(t, err, test.ShouldBeNil)
		fakeMotor := &fakemotor.Motor{}
		b.Digitals["a"] = &board.BasicDigitalInterrupt{}
		e := &encoder.SingleEncoder{I: b.Digitals["a"], CancelCtx: context.Background()}
		e.AttachDirectionalAwareness(&fakeDirectionAware{m: fakeMotor})
		e.Start(context.Background())

		m, err := WrapMotorWithEncoder(
			context.Background(),
			e,
			config.Component{Name: "motor1"},
			Config{
				TicksPerRotation: 100,
				MaxRPM:           60,
			},
			fakeMotor,
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		_, ok := m.(*EncodedMotor)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, utils.TryClose(context.Background(), m), test.ShouldBeNil)
	})

	t.Run("wrap motor with hall encoder", func(t *testing.T) {
		b, err := fakeboard.NewBoard(context.Background(), config.Component{ConvertedAttributes: &fakeboard.Config{}}, logger)
		test.That(t, err, test.ShouldBeNil)
		fakeMotor := &fakemotor.Motor{}
		b.Digitals["a"] = &board.BasicDigitalInterrupt{}
		b.Digitals["b"] = &board.BasicDigitalInterrupt{}
		e := &encoder.IncrementalEncoder{A: b.Digitals["a"], B: b.Digitals["b"], CancelCtx: context.Background()}
		e.Start(context.Background())

		m, err := WrapMotorWithEncoder(
			context.Background(),
			e,
			config.Component{Name: "motor1"},
			Config{
				TicksPerRotation: 100,
				MaxRPM:           60,
			},
			fakeMotor,
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		_, ok := m.(*EncodedMotor)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, utils.TryClose(context.Background(), m), test.ShouldBeNil)
	})
}

func TestDirFlipMotor(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg := Config{TicksPerRotation: 100, MaxRPM: 100, DirectionFlip: true}
	dirflipFakeMotor := &fakemotor.Motor{
		MaxRPM:           100,
		Logger:           logger,
		TicksPerRotation: 100,
		DirFlip:          true,
	}
	interrupt := &board.BasicDigitalInterrupt{}

	e := &encoder.SingleEncoder{I: interrupt, CancelCtx: context.Background()}
	e.AttachDirectionalAwareness(&fakeDirectionAware{m: dirflipFakeMotor})
	e.Start(context.Background())
	dirFMotor, err := NewEncodedMotor(config.Component{}, cfg, dirflipFakeMotor, e, logger)
	test.That(t, err, test.ShouldBeNil)
	_dirFMotor, ok := dirFMotor.(*EncodedMotor)
	test.That(t, ok, test.ShouldBeTrue)

	t.Run("Direction flip RPM + | REV + ", func(t *testing.T) {
		test.That(t, _dirFMotor.goForInternal(context.Background(), 1000, 1), test.ShouldBeNil)
		test.That(t, dirflipFakeMotor.PowerPct(), test.ShouldBeLessThan, 0)
		test.That(t, dirflipFakeMotor.Direction(), test.ShouldEqual, -1)
	})

	t.Run("Direction flip RPM - | REV + ", func(t *testing.T) {
		test.That(t, _dirFMotor.goForInternal(context.Background(), -1000, 1), test.ShouldBeNil)
		test.That(t, dirflipFakeMotor.PowerPct(), test.ShouldBeGreaterThan, 0)
		test.That(t, dirflipFakeMotor.Direction(), test.ShouldEqual, 1)
	})
}
