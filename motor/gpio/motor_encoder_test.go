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
	pb "go.viam.com/core/proto/api/v1"
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
	isOn, err := motor.IsOn(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isOn, test.ShouldBeFalse)
	supported, err := motor.PositionSupported(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, supported, test.ShouldBeTrue)

	test.That(t, motor.IsRegulated(), test.ShouldBeFalse)
	motor.SetRegulated(true)
	test.That(t, motor.IsRegulated(), test.ShouldBeTrue)
	motor.SetRegulated(false)
	test.That(t, motor.IsRegulated(), test.ShouldBeFalse)

	// when we go forward things work
	test.That(t, motor.Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, .01), test.ShouldBeNil)
	test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
	test.That(t, real.PowerPct(), test.ShouldEqual, float32(.01))

	// stop
	test.That(t, motor.Off(context.Background()), test.ShouldBeNil)
	test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED)

	// now test basic control
	test.That(t, motor.GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1000, 1), test.ShouldBeNil)
	test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
	test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, float32(0))

	atStart := motor.RPMMonitorCalls()
	testutils.WaitForAssertion(t, func(t testing.TB) {
		test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
		test.That(t, real.PowerPct(), test.ShouldEqual, float32(1))
	})

	test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
	test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
	test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED)
	})

	// when we're in the middle of a GoFor and then call Go, don't turn off
	test.That(t, motor.GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1000, 1), test.ShouldBeNil)
	test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
	test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, float32(0))

	testutils.WaitForAssertion(t, func(t testing.TB) {
		test.That(t, real.PowerPct(), test.ShouldEqual, float32(1))
	})

	// we didn't hit the set point
	test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
	test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)

	// go to non controlled
	motor.Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, .25)

	// go far!
	test.That(t, interrupt.Ticks(context.Background(), 1000, nowNanosTest()), test.ShouldBeNil)

	// we should still be moving at the previous force
	testutils.WaitForAssertion(t, func(t testing.TB) {
		test.That(t, real.PowerPct(), test.ShouldEqual, float32(.25))
		test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
	})

	test.That(t, motor.Off(context.Background()), test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		pos, err := motor.Position(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 11.99)
	})

	// same thing, but backwards
	test.That(t, motor.GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 1000, 1), test.ShouldBeNil)
	test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
	test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, float32(0))
	atStart = motor.RPMMonitorCalls()
	testutils.WaitForAssertion(t, func(t testing.TB) {
		test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
		test.That(t, real.PowerPct(), test.ShouldEqual, float32(1))
	})

	test.That(t, interrupt.Ticks(context.Background(), 99, nowNanosTest()), test.ShouldBeNil)
	testutils.WaitForAssertion(t, func(t testing.TB) {
		test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
	})
	test.That(t, interrupt.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
	testutils.WaitForAssertion(t, func(t testing.TB) {
		test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED)
	})

	// test go for without a rotation limit
	test.That(t, motor.GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1000, 0), test.ShouldBeNil)
	atStart = motor.RPMMonitorCalls()
	testutils.WaitForAssertion(t, func(t testing.TB) {
		test.That(t, motor.RPMMonitorCalls(), test.ShouldBeGreaterThan, atStart+10)
		test.That(t, real.PowerPct(), test.ShouldBeGreaterThan, float32(.5))
	})
	test.That(t, motor.Off(context.Background()), test.ShouldBeNil)

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

	test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil) // bounce, we should do nothing
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

	// start going backwards
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

	// do a GoFor and make sure we stop
	t.Run("GoFor", func(t *testing.T) {
		undo := SetRPMSleepDebug(1, false)
		defer undo()

		err := motor.GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 100, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)

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
			test.That(t, real.Direction(), test.ShouldNotEqual, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD)
		})

	})

	t.Run("GoFor-backwards", func(t *testing.T) {
		undo := SetRPMSleepDebug(1, false)
		defer undo()

		err := motor.GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 100, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, real.Direction(), test.ShouldEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.PowerPct(), test.ShouldEqual, 1.0)
		})

		for x := 0; x < 100; x++ {
			test.That(t, encoderA.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), false, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderA.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
			test.That(t, encoderB.Tick(context.Background(), true, nowNanosTest()), test.ShouldBeNil)
		}

		testutils.WaitForAssertion(t, func(t testing.TB) {
			test.That(t, real.Direction(), test.ShouldNotEqual, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD)
		})

	})

}

func TestWrapMotorWithEncoder(t *testing.T) {
	logger := golog.NewTestLogger(t)
	real := &fake.Motor{}

	// don't wrap with no encoder
	m, err := WrapMotorWithEncoder(context.Background(), nil, config.Component{Name: "motor1"}, motor.Config{}, real, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m, test.ShouldEqual, real)
	test.That(t, utils.TryClose(m), test.ShouldBeNil)

	// enforce need TicksPerRotation
	m, err = WrapMotorWithEncoder(context.Background(), nil, config.Component{Name: "motor1"}, motor.Config{Encoder: "a"}, real, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, m, test.ShouldBeNil)
	test.That(t, utils.TryClose(m), test.ShouldBeNil)

	b, err := fake.NewBoard(context.Background(), config.Component{
		ConvertedAttributes: &board.Config{},
	}, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)

	// enforce need encoder
	m, err = WrapMotorWithEncoder(context.Background(), b, config.Component{Name: "motor1"}, motor.Config{Encoder: "a", TicksPerRotation: 100}, real, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, m, test.ShouldBeNil)
	test.That(t, utils.TryClose(m), test.ShouldBeNil)

	b.Digitals["a"] = &board.BasicDigitalInterrupt{}
	m, err = WrapMotorWithEncoder(context.Background(), b, config.Component{Name: "motor1"}, motor.Config{Encoder: "a", TicksPerRotation: 100}, real, logger)
	test.That(t, err, test.ShouldBeNil)
	_, ok := m.(*EncodedMotor)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, utils.TryClose(m), test.ShouldBeNil)

	// enforce need encoder b
	m, err = WrapMotorWithEncoder(context.Background(), b, config.Component{Name: "motor1"}, motor.Config{Encoder: "a", TicksPerRotation: 100, EncoderB: "b"}, real, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, m, test.ShouldBeNil)
	test.That(t, utils.TryClose(m), test.ShouldBeNil)

	m, err = WrapMotorWithEncoder(context.Background(), b, config.Component{Name: "motor1"}, motor.Config{Encoder: "a", EncoderB: "b", TicksPerRotation: 100}, real, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, m, test.ShouldBeNil)
	test.That(t, utils.TryClose(m), test.ShouldBeNil)
}
