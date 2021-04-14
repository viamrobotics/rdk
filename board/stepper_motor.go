package board

import (
	"context"
	"fmt"
	"math"
	"time"

	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

const minute = 1 * time.Minute
const defaultRPM = 60

func NewStepperMotor(b GPIOBoard, pins map[string]string, mc MotorConfig, logger golog.Logger) (*StepperMotor, error) {

	// Wait is the number of MICROseconds between each step to give the desired RPM

	stop := make(map[int](chan bool))

	// Technically you can have the two jumpers set to keep ENA and ENB always-on on a dual H-bridge
	// Note that this may cause unwanted heat buildup on the H-bridge, so we require PWM control
	// Two PWM pins are used, each control the on/off state of one side of a dual H-bridge
	// We use both sides for a stepper motor, so they should either both be on or both off
	// Since we step manually, these should not be actually used for PWM, with motor speed instead
	// being controlled via the timing of the ABCD pins. Otherwise we risk partial steps and getting the
	// motor coils into a bad state.
	m := &StepperMotor{
		cfg:    mc,
		Board:  b,
		A:      pins["a"],
		B:      pins["b"],
		C:      pins["c"],
		D:      pins["d"],
		PWMs:   []string{pins["pwm"]},
		on:     false,
		stop:   stop,
		logger: logger,
		nextID: 0,
	}
	if len(pins) == 6 {
		// The two PWM inputs can be controlled by one pin whose output is forked, above, or two individual pins
		// Benefit of two individual pins is that the H-bridge can be plugged directly into a Pi without
		// the use of a breadboard
		m.PWMs = append(m.PWMs, pins["pwmb"])
	}
	return m, nil
}

// 4-wire stepper motors have various coils that must be activated in the correct sequence
// https://www.raspberrypi.org/forums/viewtopic.php?t=55580
func stepSequence() [][]bool {
	return [][]bool{
		{true, false, true, false},
		{false, true, true, false},
		{false, true, false, true},
		{true, false, false, true},
	}
}

type StepperMotor struct {
	cfg        MotorConfig
	Board      GPIOBoard
	PWMs       []string
	A, B, C, D string
	steps      int
	on         bool
	stop       map[int]chan bool
	logger     golog.Logger
	nextID     int
}

// TODO(pl): One nice feature of stepper motors is their ability to hold a stationary position and remain torqued.
//           This should eventually be a supported feature.
func (m *StepperMotor) Position(ctx context.Context) (float64, error) {
	return float64(m.steps / m.cfg.TicksPerRotation), nil
}

func (m *StepperMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// TODO(pl): Implement this feature once we have a driver board allowing PWM control
func (m *StepperMotor) Force(ctx context.Context, force byte) error {
	return fmt.Errorf("force not supported for stepper motors on dual H-bridges")
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
func (m *StepperMotor) step(ctx context.Context, d pb.DirectionRelative, wait time.Duration) error {
	seq := stepSequence()
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED:
		return m.Off(ctx)
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		for i := 0; i < len(seq); i++ {
			err := m.setStep(ctx, seq[i])
			if err != nil {
				return err
			}
			m.steps++
			// time.Sleep between each setStep() call is the best way to adjust motor speed
			// See the comment above in NewStepperMotor() for why to not use PWM
			time.Sleep(wait)
		}
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		for i := len(seq) - 1; i >= 0; i-- {
			err := m.setStep(ctx, seq[i])
			if err != nil {
				return err
			}
			m.steps--
			time.Sleep(wait)
		}
	}
	return nil
}

// Use this to launch a goroutine that will rotate in a direction while listening on a channel for Off()
func (m *StepperMotor) Go(ctx context.Context, d pb.DirectionRelative, force byte) error {
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED {
		return m.Off(ctx)
	}
	if m.on {
		err := m.Off(ctx)
		if err != nil {
			return err
		}
	}
	err := m.turnOn(ctx)
	if err != nil {
		return err
	}

	thisID := m.nextID
	m.stop[thisID] = make(chan bool)
	m.nextID++

	wait := time.Duration(float64(minute.Microseconds())/(float64(m.cfg.TicksPerRotation)*defaultRPM)) * time.Microsecond

	// Turn in the specified direction until something says not to
	go func() {
		for {
			done := false
			select {
			case <-m.stop[thisID]:
				done = true
			default:
				err := m.step(ctx, d, wait)
				if err != nil {
					m.logger.Warnf("error performing gofor step: %s", err)
				}
			}
			if done {
				break
			}
		}
		err := m.powerOff(ctx)
		if err != nil {
			m.logger.Warnf("error turning off: %s", err)
		}
		delete(m.stop, thisID)
	}()
	return nil
}

// Turn in the given direction the given number of times at the given speed. Does not block
func (m *StepperMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	// Set our wait time off of the specified RPM
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED {
		return m.Off(ctx)
	}
	if m.on {
		err := m.Off(ctx)
		if err != nil {
			return err
		}
	}
	err := m.turnOn(ctx)
	if err != nil {
		return err
	}

	thisID := m.nextID
	m.stop[thisID] = make(chan bool)
	m.nextID++

	wait := time.Duration(float64(minute.Microseconds())/(float64(m.cfg.TicksPerRotation)*rpm)) * time.Microsecond
	steps := int(math.Abs(rotations * float64(m.cfg.TicksPerRotation)))

	go func() {
		// Rotate the specified number of rotations. We step in increments of 4.
		for i := steps; i > 0; i -= 4 {
			done := false
			select {
			case <-m.stop[thisID]:
				done = true
			default:
				err := m.step(ctx, d, wait)
				if err != nil {
					m.logger.Warnf("error performing gofor step: %s", err)
				}
			}
			if done {
				break
			}
		}
		err := m.powerOff(ctx)
		if err != nil {
			m.logger.Warnf("error turning off: %s", err)
		}
		delete(m.stop, thisID)
	}()
	return nil
}

func (m *StepperMotor) IsOn(ctx context.Context) (bool, error) {
	return m.on, nil
}

// Turn on power to the motor
func (m *StepperMotor) turnOn(ctx context.Context) error {
	m.on = true
	var err error

	for _, pwmPin := range m.PWMs {
		err = multierr.Combine(
			err,
			m.Board.GPIOSet(pwmPin, true),
		)
	}
	return err
}

// Turn off power to the motor without sending stop signals to channels
func (m *StepperMotor) powerOff(ctx context.Context) error {
	m.on = false
	var err error

	for _, pwmPin := range m.PWMs {
		err = multierr.Combine(
			err,
			m.Board.GPIOSet(pwmPin, false),
		)
	}
	return err
}

// Turn off power to the motor and stop all movement
func (m *StepperMotor) Off(ctx context.Context) error {
	m.stopRunningThreads(ctx)
	return m.powerOff(ctx)
}

// Tell all running threads to stop movement. Does not turn off torque.
func (m *StepperMotor) stopRunningThreads(ctx context.Context) {
	for k := range m.stop {
		m.stop[k] <- true
	}
}
