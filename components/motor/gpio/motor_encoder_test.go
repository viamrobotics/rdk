package gpio

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	fakeboard "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/encoder/incremental"
	"go.viam.com/rdk/components/encoder/single"
	"go.viam.com/rdk/components/motor"
	fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

// setupMotorWithEncoder(encType string) {}

func nowNanosTest() uint64 {
	return uint64(time.Now().UnixNano())
}

type fakeDirectionAware struct {
	m *fakemotor.Motor
}

func (f *fakeDirectionAware) DirectionMoving() int64 {
	return int64(f.m.Direction())
}

func MakeSingleBoard(t *testing.T) *fakeboard.Board {
	interrupt, _ := fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{
		Name: "10",
		Pin:  "10",
		Type: "basic",
	})

	interrupts := map[string]*fakeboard.DigitalInterruptWrapper{
		"10": interrupt,
	}

	b := fakeboard.Board{
		GPIOPins: map[string]*fakeboard.GPIOPin{},
		Digitals: interrupts,
	}

	return &b
}

func MakeIncrementalBoard(t *testing.T) *fakeboard.Board {
	interrupt11, _ := fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{
		Name: "11",
		Pin:  "11",
		Type: "basic",
	})

	interrupt13, _ := fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{
		Name: "13",
		Pin:  "13",
		Type: "basic",
	})

	interrupts := map[string]*fakeboard.DigitalInterruptWrapper{
		"11": interrupt11,
		"13": interrupt13,
	}

	b := fakeboard.Board{
		GPIOPins: map[string]*fakeboard.GPIOPin{},
		Digitals: interrupts,
	}

	return &b
}

func TestMotorEncoder1(t *testing.T) {
	t.Skip()
	logger := golog.NewTestLogger(t)

	cfg := Config{TicksPerRotation: 100, MaxRPM: 100}
	fakeMotor := &fakemotor.Motor{
		MaxRPM:           100,
		Logger:           logger,
		TicksPerRotation: 100,
		OpMgr:            operation.NewSingleOperationManager(),
	}
	interrupt := &board.BasicDigitalInterrupt{}

	ctx := context.Background()
	b := MakeSingleBoard(t)
	deps := make(resource.Dependencies)
	deps[board.Named("main")] = b

	ic := single.Config{
		BoardName: "main",
		Pins:      single.Pin{I: "10"},
	}

	rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}
	e, err := single.NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	enc := e.(*single.Encoder)
	defer enc.Close(context.Background())

	enc.AttachDirectionalAwareness(&fakeDirectionAware{m: fakeMotor})
	dirFMotor, err := NewEncodedMotor(resource.Config{}, cfg, fakeMotor, e, logger)
	test.That(t, err, test.ShouldBeNil)
	defer dirFMotor.Close(context.Background())
	motorDep, ok := dirFMotor.(*EncodedMotor)
	defer motorDep.Close(context.Background())
	test.That(t, ok, test.ShouldBeTrue)
	defer func() {
		test.That(t, motorDep.Close(context.Background()), test.ShouldBeNil)
	}()

	t.Run("encoded motor testing the basics", func(t *testing.T) {
		isOn, powerPct, err := motorDep.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isOn, test.ShouldBeFalse)
		test.That(t, powerPct, test.ShouldEqual, 0.0)
		properties, err := motorDep.Properties(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, properties.PositionReporting, test.ShouldBeTrue)
	})

	t.Run("encoded motor testing SetPower", func(t *testing.T) {
		test.That(t, motorDep.SetPower(context.Background(), .01, nil), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldEqual, .01)
	})

	t.Run("encoded motor testing Stop", func(t *testing.T) {
		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 0)
	})

	t.Run("encoded motor cannot go at 0 RPM", func(t *testing.T) {
		test.That(t, motorDep.GoFor(context.Background(), 0, 1, nil), test.ShouldBeError, motor.NewZeroRPMError())
	})

	t.Run("encoded motor testing SetPower interrupt GoFor", func(t *testing.T) {
		test.That(t, motorDep.goForInternal(context.Background(), 1000, 1000), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldBeGreaterThan, float32(0))

		errChan := make(chan error)
		go func() {
			ticksErr := interrupt.Ticks(context.Background(), 99, nowNanosTest())
			errChan <- ticksErr
		}()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.PowerPct(), test.ShouldEqual, float32(1))
		})
		motorDep.SetPower(context.Background(), -0.25, nil)
		receivedErr := <-errChan
		test.That(t, receivedErr, test.ShouldBeNil)
		pos, err := motorDep.Position(context.Background(), nil)
		// should not have reached the final position intended by the
		// goForInternal call
		test.That(t, pos, test.ShouldBeLessThan, 1000)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, -1)
	})

	t.Run("encoded motor testing Go (non controlled)", func(t *testing.T) {
		motorDep.ResetZeroPosition(context.Background(), 0, nil)
		motorDep.SetPower(context.Background(), .25, nil)
		test.That(t, interrupt.Ticks(context.Background(), 1000, nowNanosTest()), test.ShouldBeNil) // go far!

		// we should still be moving at the previous force
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.PowerPct(), test.ShouldEqual, float32(.25))
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 1)
		})

		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (RPM + | REV +)", func(t *testing.T) {
		test.That(t, motorDep.goForInternal(context.Background(), 1000, 1), test.ShouldBeNil)
		test.That(t, motorDep.DirectionMoving(), test.ShouldEqual, 1)

		test.That(t, enc.I.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 1)
		})

		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)

		test.That(t, motorDep.goForInternal(context.Background(), 1000, 1), test.ShouldBeNil)
		atStart := motorDep.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, motorDep.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(tb, fakeMotor.PowerPct(), test.ShouldBeGreaterThan, 0.5)
		})

		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (RPM - | REV +)", func(t *testing.T) {
		test.That(t, motorDep.goForInternal(context.Background(), -1000, 1), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, -1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldBeLessThan, 0)

		test.That(t, enc.I.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, -1)
		})

		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)

		test.That(t, motorDep.goForInternal(context.Background(), -1000, 1), test.ShouldBeNil)
		atStart := motorDep.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, motorDep.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(tb, fakeMotor.PowerPct(), test.ShouldEqual, -1)
		})
		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (RPM + | REV -)", func(t *testing.T) {
		test.That(t, motorDep.goForInternal(context.Background(), 1000, -1), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, -1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldBeLessThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, -1)
		})

		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)

		test.That(t, motorDep.goForInternal(context.Background(), 1000, -1), test.ShouldBeNil)
		atStart := motorDep.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, motorDep.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(tb, fakeMotor.PowerPct(), test.ShouldEqual, -1)
		})

		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)
	})

	t.Run("encoded motor testing GoFor (RPM - | REV -)", func(t *testing.T) {
		test.That(t, motorDep.goForInternal(context.Background(), -1000, -1), test.ShouldBeNil)
		test.That(t, fakeMotor.Direction(), test.ShouldEqual, 1)
		test.That(t, fakeMotor.PowerPct(), test.ShouldBeGreaterThan, 0)

		test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, fakeMotor.Direction(), test.ShouldEqual, 1)
		})

		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)

		test.That(t, motorDep.goForInternal(context.Background(), -1000, -1), test.ShouldBeNil)
		atStart := motorDep.RPMMonitorCalls()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, motorDep.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
			test.That(tb, fakeMotor.PowerPct(), test.ShouldBeGreaterThan, 0.5)
		})

		test.That(t, motorDep.Stop(context.Background(), nil), test.ShouldBeNil)
	})

	t.Run("Ensure stop called when gofor is interrupted", func(t *testing.T) {
		ctx := context.Background()
		var wg sync.WaitGroup
		ctx, cancel := context.WithCancel(ctx)
		wg.Add(1)
		go func() {
			motorDep.GoFor(ctx, 100, 100, map[string]interface{}{})
			wg.Done()
		}()
		cancel()
		wg.Wait()

		test.That(t, ctx.Err(), test.ShouldNotBeNil)
		test.That(t, motorDep.state.goalRPM, test.ShouldEqual, 0)
	})
}

func TestMotorEncoderIncremental(t *testing.T) {
	t.Skip()
	logger := golog.NewTestLogger(t)

	type testHarness struct {
		Encoder   *incremental.Encoder
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
			OpMgr:            operation.NewSingleOperationManager(),
		}

		ctx := context.Background()
		b := MakeIncrementalBoard(t)
		deps := make(resource.Dependencies)
		deps[board.Named("main")] = b

		ic := incremental.Config{
			BoardName: "main",
			Pins:      incremental.Pins{A: "11", B: "13"},
		}

		rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}
		e, err := incremental.NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc := e.(*incremental.Encoder)

		motorIfc, err := NewEncodedMotor(resource.Config{}, cfg, fakeMotor, enc, logger)
		test.That(t, err, test.ShouldBeNil)

		motor, ok := motorIfc.(*EncodedMotor)
		test.That(t, ok, test.ShouldBeTrue)

		motor.RPMMonitorStart()
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0.0)
		})

		return testHarness{
			enc,
			enc.A,
			enc.B,
			fakeMotor,
			motor,
			func() { test.That(t, motor.Close(context.Background()), test.ShouldBeNil) },
		}
	}

	t.Run("motor encoder no motion", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		enc := th.Encoder
		encB := th.EncoderB

		test.That(t, encB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // bounce, we should do nothing
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0)
		})
	})

	t.Run("motor encoder move forward", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		enc := th.Encoder
		encA := th.EncoderA
		encB := th.EncoderB

		// this should do nothing because it's the initial state
		test.That(t, encA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0)
		})
		test.That(t, encB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // we go from state 00 -> 10
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 1)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 10 -> 11
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 2)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 11 -> 01
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 3)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 01 -> 00
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 4)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 2)
			test.That(tb, err, test.ShouldBeNil)
		})
	})

	t.Run("motor encoder move backward", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		enc := th.Encoder
		encA := th.EncoderA
		encB := th.EncoderB

		// this should do nothing because it's the initial state
		test.That(t, encA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0)
		})
		test.That(t, encB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // we go from state 00 -> 10
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 1)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 10 -> 11
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 2)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 11 -> 01
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 3)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 01 -> 00
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 4)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 2)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // we go from state 00 -> 01
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 3)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil) // 01 -> 11
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 2)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 1)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 11 -> 10
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 1)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 0)
			test.That(tb, err, test.ShouldBeNil)
		})

		test.That(t, encB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // 10 -> 00
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos := enc.RawPosition()
			test.That(tb, pos, test.ShouldEqual, 0)
			posFl, _, err := enc.Position(context.Background(), encoder.PositionTypeUnspecified, nil)
			pos = int64(posFl)
			test.That(tb, pos, test.ShouldEqual, 0)
			test.That(tb, err, test.ShouldBeNil)
		})
	})

	t.Run("motor encoder test GoFor (forward)", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		encA := th.EncoderA
		encB := th.EncoderB
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
			test.That(t, encB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, realMotor.Direction(), test.ShouldNotEqual, 1)
		})
	})

	t.Run("motor encoder test GoFor (backwards)", func(t *testing.T) {
		th := setup(t)
		defer th.Teardown()

		encA := th.EncoderA
		encB := th.EncoderB
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
			test.That(t, encA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, realMotor.Direction(), test.ShouldNotEqual, -1)
		})
	})
}

func TestWrapMotorWithEncoder(t *testing.T) {
	t.Skip()
	logger := golog.NewTestLogger(t)

	t.Run("wrap motor no encoder", func(t *testing.T) {
		fakeMotor := &fakemotor.Motor{
			OpMgr: operation.NewSingleOperationManager(),
		}
		m, err := WrapMotorWithEncoder(
			context.Background(),
			nil,
			resource.Config{Name: "motor1"}, Config{},
			fakeMotor,
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, m, test.ShouldEqual, fakeMotor)
		test.That(t, m.Close(context.Background()), test.ShouldBeNil)
	})

	t.Run("wrap motor with single encoder", func(t *testing.T) {
		b := MakeSingleBoard(t)
		fakeMotor := &fakemotor.Motor{
			OpMgr: operation.NewSingleOperationManager(),
		}
		b.Digitals["a"], _ = fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{
			Type: "basic",
		})

		ctx := context.Background()
		deps := make(resource.Dependencies)
		deps[board.Named("main")] = b

		ic := single.Config{
			BoardName: "main",
			Pins:      single.Pin{I: "10"},
		}

		rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}
		e, err := single.NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)
		enc := e.(*single.Encoder)
		defer enc.Close(context.Background())

		enc.AttachDirectionalAwareness(&fakeDirectionAware{m: fakeMotor})
		m, err := WrapMotorWithEncoder(
			context.Background(),
			e,
			resource.Config{Name: "motor1"},
			Config{
				TicksPerRotation: 100,
				MaxRPM:           60,
			},
			fakeMotor,
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(context.Background())
		_, ok := m.(*EncodedMotor)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, m.Close(context.Background()), test.ShouldBeNil)
	})

	t.Run("wrap motor with hall encoder", func(t *testing.T) {
		b := MakeIncrementalBoard(t)
		fakeMotor := &fakemotor.Motor{
			OpMgr: operation.NewSingleOperationManager(),
		}
		b.Digitals["a"], _ = fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{
			Type: "basic",
		})
		b.Digitals["b"], _ = fakeboard.NewDigitalInterruptWrapper(board.DigitalInterruptConfig{
			Type: "basic",
		})

		ctx := context.Background()
		deps := make(resource.Dependencies)
		deps[board.Named("main")] = b

		ic := incremental.Config{
			BoardName: "main",
			Pins:      incremental.Pins{A: "11", B: "13"},
		}

		rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}

		e, err := incremental.NewIncrementalEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
		test.That(t, err, test.ShouldBeNil)

		m, err := WrapMotorWithEncoder(
			context.Background(),
			e,
			resource.Config{Name: "motor1"},
			Config{
				TicksPerRotation: 100,
				MaxRPM:           60,
			},
			fakeMotor,
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		defer m.Close(context.Background())
		_, ok := m.(*EncodedMotor)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, m.Close(context.Background()), test.ShouldBeNil)
	})
}

func TestDirFlipMotor(t *testing.T) {
	t.Skip()
	logger := golog.NewTestLogger(t)
	cfg := Config{TicksPerRotation: 100, MaxRPM: 100, DirectionFlip: true}
	dirflipFakeMotor := &fakemotor.Motor{
		MaxRPM:           100,
		Logger:           logger,
		TicksPerRotation: 100,
		DirFlip:          true,
		OpMgr:            operation.NewSingleOperationManager(),
	}
	defer dirflipFakeMotor.Close(context.Background())

	ctx := context.Background()
	b := MakeSingleBoard(t)
	deps := make(resource.Dependencies)
	deps[board.Named("main")] = b

	ic := single.Config{
		BoardName: "main",
		Pins:      single.Pin{I: "10"},
	}

	rawcfg := resource.Config{Name: "enc1", ConvertedAttributes: &ic}
	e, err := single.NewSingleEncoder(ctx, deps, rawcfg, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	enc := e.(*single.Encoder)
	defer enc.Close(context.Background())

	enc.AttachDirectionalAwareness(&fakeDirectionAware{m: dirflipFakeMotor})
	dirFMotor, err := NewEncodedMotor(resource.Config{}, cfg, dirflipFakeMotor, e, logger)
	test.That(t, err, test.ShouldBeNil)
	defer dirFMotor.Close(context.Background())
	_dirFMotor, ok := dirFMotor.(*EncodedMotor)
	test.That(t, ok, test.ShouldBeTrue)
	defer _dirFMotor.Close(context.Background())

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
