package board

import (
	"testing"
	"time"

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
	assert.Equal(t, false, motor.IsOn())

	assert.Equal(t, false, motor.isRegulated())
	motor.setRegulated(true)
	assert.Equal(t, true, motor.isRegulated())
	motor.setRegulated(false)
	assert.Equal(t, false, motor.isRegulated())

	// when we go forward things work
	assert.Nil(t, motor.Go(DirForward, 16))
	assert.Equal(t, DirForward, real.d)
	assert.Equal(t, byte(16), real.force)

	// stop
	assert.Nil(t, motor.Off())
	assert.Equal(t, DirNone, real.d)

	// now test basic control
	assert.Nil(t, motor.GoFor(DirForward, 1000, 1))
	assert.Equal(t, DirForward, real.d)
	assert.Less(t, byte(0), real.force)

	time.Sleep(20 * time.Millisecond)
	assert.Less(t, int64(10), motor.rpmMonitorCalls)
	assert.Equal(t, byte(255), real.force)

	encoder.ticks(99, nowNanosTest())
	assert.Equal(t, DirForward, real.d)
	encoder.Tick(true, nowNanosTest())
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, DirNone, real.d)

	// when we're in the middle of a GoFor and then call Go, don't turn off
	assert.Nil(t, motor.GoFor(DirForward, 1000, 1))
	assert.Equal(t, DirForward, real.d)
	assert.Less(t, byte(0), real.force)

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, byte(255), real.force)

	// we didn't hit the set point
	encoder.ticks(99, nowNanosTest())
	assert.Equal(t, DirForward, real.d)

	// go to non controlled
	motor.Go(DirForward, 64)

	// go far!
	encoder.ticks(1000, nowNanosTest())

	// we should still be moving at the previous force
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, byte(64), real.force)
	assert.Equal(t, DirForward, real.d)

	assert.Nil(t, motor.Off())

	// same thing, but backwards
	assert.Nil(t, motor.GoFor(DirBackward, 1000, 1))
	assert.Equal(t, DirBackward, real.d)
	assert.Less(t, byte(0), real.force)

	time.Sleep(20 * time.Millisecond)
	assert.Less(t, int64(10), motor.rpmMonitorCalls)
	assert.Equal(t, byte(255), real.force)

	encoder.ticks(99, nowNanosTest())
	assert.Equal(t, DirBackward, real.d)
	encoder.Tick(true, nowNanosTest())
	assert.Equal(t, DirNone, real.d)

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

	motor.rpmMonitorStart()
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, int64(0), motor.curPosition)

	encoderA.Tick(true, nowNanosTest())
	encoderB.Tick(true, nowNanosTest())
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int64(-1), motor.curPosition)

	encoderB.Tick(true, nowNanosTest())
	encoderA.Tick(true, nowNanosTest())
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int64(0), motor.curPosition)

	encoderB.Tick(false, nowNanosTest())
	encoderB.Tick(true, nowNanosTest())
	encoderA.Tick(true, nowNanosTest())
	time.Sleep(210 * time.Millisecond)
	assert.Equal(t, int64(1), motor.curPosition)

}
