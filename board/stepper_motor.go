package board

import (
	"context"
	"fmt"
	"math"
	"time"

	pb "go.viam.com/robotcore/proto/api/v1"

	"go.uber.org/multierr"
)

func NewStepperMotor(b GPIOBoard, pins map[string]string, mc MotorConfig) (*StepperMotor, error) {
	
	defaultRPM := 10
	
	// Wait is the number of MICROseconds between each step to give the desired RPM
	wait := time.Duration(60000000/(mc.TicksPerRotation * defaultRPM)) * time.Microsecond
	
	// Technically you can have the two jumpers set to keep ENA and ENB always-on on a dual H-bridge
	// Note that this may cause unwanted heat buildup on the H-bridge, so we require PWM control
	// Two PWM pins are used, each control the on/off state of one side of a dual H-bridge
	// We use both sides for a stepper motor, so they should either both be on or both off
	// Since we step manually, these should not be actually used for PWM, with motor speed instead
	// being controlled via the timing of the ABCD pins. Otherwise we risk partial steps and getting the
	// motor coils into a bad state.
	m := &StepperMotor{
		cfg:        mc,
		Board:      b,
		wait:       wait,
		A:          pins["a"],
		B:          pins["b"],
		C:          pins["c"],
		D:          pins["d"],
		PWMs:       []string{pins["pwm"]},
		on:         false,
	}
	if len(pins) == 6 {
		// The two PWM inputs can be controlled by one pin whwose output is forked, above, or two individual pins
		// Benefit of two individual pins is that the H-bridge can be plugged directly into a Pi without
		// the use of a breadboard
		m.PWMs = append(m.PWMs, pins["pwmb"])
	}
	return m, nil
}

// 4-wire stepper motors have various coils that must be activated in the correct sequence
// https://www.raspberrypi.org/forums/viewtopic.php?t=55580
func stepSequence() [][]bool{
	return [][]bool{
		{true, false, true, false},
		{false, true, true, false},
		{false, true, false, true},
		{true, false, false, true},
	}
}

type StepperMotor struct {
	cfg         MotorConfig
	Board       GPIOBoard
	wait        time.Duration
	PWMs        []string
	A, B, C, D  string
	steps       int
	on          bool
}

// TODO(pl): One nice feature of stepper motors is their ability to hold a stationary position and remain torqued,
//           unlike a DC motor. This should eventually be a supported feature.
func (m *StepperMotor) Position(ctx context.Context) (float64, error) {
	return float64(m.steps), nil
}

func (m *StepperMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// TODO(pl): Implement this feature once we have a driver board allowing PWM control 
func (m *StepperMotor) Force(ctx context.Context, force byte) error {
	return fmt.Errorf("Force not supported for stepper motors on dual H-bridges")
}

func (m *StepperMotor) setStep(ctx context.Context, pins []bool) error {
	return multierr.Combine(
		m.Board.GPIOSet(m.A, pins[0]),
		m.Board.GPIOSet(m.B, pins[1]),
		m.Board.GPIOSet(m.C, pins[2]),
		m.Board.GPIOSet(m.D, pins[3]),
	)
}

// This will power on the motor if necessary, and make one full step sequence (4 steps) in the specified direction
func (m *StepperMotor) step(ctx context.Context, d pb.DirectionRelative) error {
	if !m.on{
		err := m.turnOn(ctx)
		if err != nil{
			return err
		}
	}
	seq := stepSequence()
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED:
		return m.Off(ctx)
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		for i := 0; i < len(seq); i++{
			err := m.setStep(ctx, seq[i])
			if err != nil{
				return err
			}
			m.steps++
			time.Sleep(m.wait)
		}
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		for i := len(seq) - 1; i >= 0; i--{
			err := m.setStep(ctx, seq[i])
			if err != nil{
				return err
			}
			m.steps--
			time.Sleep(m.wait)
		}
	}
	return nil
}

// TODO(pl): use this to launch a goroutine that will rotate in a direction while listening on a channel for Off()
func (m *StepperMotor) Go(ctx context.Context, d pb.DirectionRelative, force byte) error {
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED{
		return m.Off(ctx)
	}
	return fmt.Errorf("unlimited rotation not yet implemented for stepper motors, please use GoFor()")
}

func (m *StepperMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	// Set our wait time off of the specified RPM
	m.wait = time.Duration(int(60000000.0/(float64(m.cfg.TicksPerRotation) * rpm))) * time.Microsecond
	
	// Rotate the specified number of steps
	// This is counting steps, not rotations. Probably this is mentally easier to reason about when making small
	// movements than very small fractions of one rotation
	for i := math.Abs(rotations); i > 0; i--{
		err := m.step(ctx, d)
		if err != nil{
			return multierr.Combine(err, m.Off(ctx))
		}
	}
	return m.Off(ctx)
}

func (m *StepperMotor) IsOn(ctx context.Context) (bool, error) {
	return m.on, nil
}

// Turn on power to the motor
func (m *StepperMotor) turnOn(ctx context.Context) error {
	m.on = true
	var err error
	
	for _, pwmPin := range(m.PWMs){
		err = multierr.Combine(
			err,
			m.Board.GPIOSet(pwmPin, true),
		)
	}
	return err
}

// Turn off power to the motor
func (m *StepperMotor) Off(ctx context.Context) error {
	m.on = false
	var err error
	
	for _, pwmPin := range(m.PWMs){
		err = multierr.Combine(
			err,
			m.Board.GPIOSet(pwmPin, false),
		)
	}
	return err
}
