package board

import (
	"context"
	"testing"
	"time"

	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"
)

func TestMotorEncoder1(t *testing.T) {
	rpmSleep = 1
	rpmDebug = false

	cfg := MotorConfig{TicksPerRotation: 100}
	real := &FakeMotor{}
	encoder := &BasicDigitalInterrupt{}

	motor := encodedMotor{
		cfg:     cfg,
		real:    real,
		encoder: encoder,
	}

	// test some basic defaults
	isOn, err := motor.IsOn(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, false, isOn)
	supported, err := motor.PositionSupported(context.Background())
	assert.Nil(t, err)
	assert.True(t, supported)

	assert.Equal(t, false, motor.isRegulated())
	motor.setRegulated(true)
	assert.Equal(t, true, motor.isRegulated())
	motor.setRegulated(false)
	assert.Equal(t, false, motor.isRegulated())

	// when we go forward things work
	assert.Nil(t, motor.Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 16))
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, real.d)
	assert.Equal(t, byte(16), real.force)

	// stop
	assert.Nil(t, motor.Off(context.Background()))
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, real.d)

	// now test basic control
	assert.Nil(t, motor.GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1000, 1))
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, real.d)
	assert.Less(t, byte(0), real.force)

	time.Sleep(20 * time.Millisecond)
	assert.Less(t, int64(10), motor.rpmMonitorCalls)
	assert.Equal(t, byte(255), real.force)

	encoder.ticks(99, nowNanosTest())
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, real.d)
	encoder.Tick(true, nowNanosTest())
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, real.d)

	// when we're in the middle of a GoFor and then call Go, don't turn off
	assert.Nil(t, motor.GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1000, 1))
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, real.d)
	assert.Less(t, byte(0), real.force)

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, byte(255), real.force)

	// we didn't hit the set point
	encoder.ticks(99, nowNanosTest())
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, real.d)

	// go to non controlled
	motor.Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 64)

	// go far!
	encoder.ticks(1000, nowNanosTest())

	// we should still be moving at the previous force
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, byte(64), real.force)
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, real.d)

	assert.Nil(t, motor.Off(context.Background()))

	// same thing, but backwards
	assert.Nil(t, motor.GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 1000, 1))
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, real.d)
	assert.Less(t, byte(0), real.force)

	time.Sleep(20 * time.Millisecond)
	assert.Less(t, int64(10), motor.rpmMonitorCalls)
	assert.Equal(t, byte(255), real.force)

	encoder.ticks(99, nowNanosTest())
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, real.d)
	encoder.Tick(true, nowNanosTest())
	assert.Equal(t, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, real.d)

}

func TestMotorEncoderHall(t *testing.T) {
	rpmSleep = 1
	rpmDebug = false

	cfg := MotorConfig{TicksPerRotation: 100}
	real := &FakeMotor{}
	encoderA := &BasicDigitalInterrupt{}
	encoderB := &BasicDigitalInterrupt{}

	motor := encodedMotor{
		cfg:      cfg,
		real:     real,
		encoder:  encoderA,
		encoderB: encoderB,
	}

	motor.rpmMonitorStart(context.Background())
	time.Sleep(20 * time.Millisecond)

	pos, err := motor.Position(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, 0.0, pos)

	encoderA.Tick(true, nowNanosTest())
	encoderB.Tick(true, nowNanosTest())
	time.Sleep(20 * time.Millisecond)
	pos, err = motor.Position(context.Background())
	assert.Nil(t, err)
	assert.InEpsilon(t, -.01, pos, .00001)

	encoderB.Tick(true, nowNanosTest())
	encoderA.Tick(true, nowNanosTest())
	time.Sleep(20 * time.Millisecond)
	pos, err = motor.Position(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, 0.0, pos)

	encoderB.Tick(false, nowNanosTest())
	encoderB.Tick(true, nowNanosTest())
	encoderA.Tick(true, nowNanosTest())
	time.Sleep(210 * time.Millisecond)
	pos, err = motor.Position(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, .01, pos)

}

func TestMotorEncoderWrap(t *testing.T) {
	logger := golog.NewTestLogger(t)
	real := &FakeMotor{}

	// don't wrap with no encoder
	m, err := WrapMotorWithEncoder(context.Background(), nil, MotorConfig{}, real, logger)
	assert.Nil(t, err)
	assert.Equal(t, real, m)

	// enforce need TicksPerRotation
	m, err = WrapMotorWithEncoder(context.Background(), nil, MotorConfig{Encoder: "a"}, real, logger)
	assert.NotNil(t, err)
	assert.Nil(t, m)

	b, err := NewFakeBoard(context.Background(), Config{}, golog.Global)
	if err != nil {
		t.Fatal(err)
	}

	// enforce need encoder
	m, err = WrapMotorWithEncoder(context.Background(), b, MotorConfig{Encoder: "a", TicksPerRotation: 100}, real, logger)
	assert.NotNil(t, err)
	assert.Nil(t, m)

	b.digitals["a"] = &BasicDigitalInterrupt{}
	m, err = WrapMotorWithEncoder(context.Background(), b, MotorConfig{Encoder: "a", TicksPerRotation: 100}, real, logger)
	assert.Nil(t, err)
	_, ok := m.(*encodedMotor)
	assert.True(t, ok)

	// enforce need encoder b
	m, err = WrapMotorWithEncoder(context.Background(), b, MotorConfig{Encoder: "a", TicksPerRotation: 100, EncoderB: "b"}, real, logger)
	assert.NotNil(t, err)
	assert.Nil(t, m)

	m, err = WrapMotorWithEncoder(context.Background(), b, MotorConfig{Encoder: "a", EncoderB: "b", TicksPerRotation: 100}, real, logger)
	assert.NotNil(t, err)
	assert.Nil(t, m)

}
