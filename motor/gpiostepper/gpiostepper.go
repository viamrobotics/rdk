package gpiostepper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"go.uber.org/multierr"

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	pb "go.viam.com/core/proto/api/v1"
)

const modelName = "gpiostepper"

func init() {
	registry.RegisterMotor(modelName, registry.Motor{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error) {
		actualBoard, motorConfig, err := getBoardFromRobotConfig(r, config)
		if err != nil {
			return nil, err
		}

		return newGPIOStepper(ctx, actualBoard, *motorConfig, logger)
	}})
	motor.RegisterConfigAttributeConverter(modelName)
}

func getBoardFromRobotConfig(r robot.Robot, config config.Component) (board.Board, *motor.Config, error) {
	motorConfig := config.ConvertedAttributes.(*motor.Config)
	if motorConfig.BoardName == "" {
		return nil, nil, errors.New("expected board name in config for motor")
	}
	b, ok := r.BoardByName(motorConfig.BoardName)
	if !ok {
		return nil, nil, fmt.Errorf("expected to find board %q", motorConfig.BoardName)
	}
	return b, motorConfig, nil
}

func newGPIOStepper(ctx context.Context, b board.Board, mc motor.Config, logger golog.Logger) (motor.Motor, error) {
	m := &gpioStepper{
		theBoard:         b,
		stepsPerRotation: mc.TicksPerRotation,
		enablePinHigh:    mc.Pins["enHigh"],
		enablePinLow:     mc.Pins["enLow"],
		stepPin:          mc.Pins["step"],
		dirPin:           mc.Pins["dir"],
		logger:           logger,
	}

	err := m.Validate()
	if err != nil {
		return nil, err
	}

	m.startThread(ctx)
	return m, nil
}

type gpioStepper struct {
	// config
	theBoard                    board.Board
	stepsPerRotation            int
	enablePinHigh, enablePinLow string
	stepPin, dirPin             string
	logger                      golog.Logger

	// state
	lock sync.Mutex

	stepPosition         int64
	threadStarted        bool
	targetStepPosition   int64
	targetStepsPerSecond int64
}

// validate if this config is valid
func (m *gpioStepper) Validate() error {
	if m.theBoard == nil {
		return errors.New("need a board for gpioStepper")
	}

	if m.stepsPerRotation == 0 {
		m.stepsPerRotation = 200
	}

	if m.stepPin == "" {
		return errors.New("need a 'step' pin for gpioStepper")
	}

	if m.dirPin == "" {
		return errors.New("need a 'dir' pin for gpioStepper")
	}

	return nil
}

// Power sets the percentage of power the motor should employ between 0-1.
func (m *gpioStepper) Power(ctx context.Context, powerPct float32) error {
	if powerPct <= .0001 {
		m.stop()
		return nil
	}

	return errors.New("gpioStepper doesn't support raw power mode")
}

// Go instructs the motor to go in a specific direction at a percentage
// of power between 0-1.
func (m *gpioStepper) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	if powerPct <= .0001 {
		m.stop()
		return nil
	}
	return errors.New("gpioStepper doesn't support raw power mode")
}

func (m *gpioStepper) startThread(ctx context.Context) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.threadStarted {
		return
	}

	m.threadStarted = true
	go m.doRun(ctx)
}

func (m *gpioStepper) doRun(ctx context.Context) {
	for {
		sleep, err := m.doCycle(ctx)

		if err != nil {
			m.logger.Info("error in gpioStepper %w", err)
		}

		if !utils.SelectContextOrWait(ctx, sleep) {
			return
		}
	}
}

func (m *gpioStepper) doCycle(ctx context.Context) (time.Duration, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.stepPosition == m.targetStepPosition {
		return 5 * time.Millisecond, nil
	}

	err := m.doStep(ctx, m.stepPosition < m.targetStepPosition)
	if err != nil {
		return time.Second, fmt.Errorf("error stepping %w", err)
	}

	return time.Duration(int64(time.Microsecond*1000*1000) / m.targetStepsPerSecond), nil
}

// have to be locked to call
func (m *gpioStepper) doStep(ctx context.Context, forward bool) error {
	err := multierr.Combine(
		m.enable(ctx, true),
		m.theBoard.GPIOSet(ctx, m.stepPin, true),
		m.theBoard.GPIOSet(ctx, m.dirPin, forward),
	)
	if err != nil {
		return err
	}

	time.Sleep(time.Microsecond) // TODO(erh): test what's actually correct here

	if forward {
		m.stepPosition++
	} else {
		m.stepPosition--
	}
	return m.theBoard.GPIOSet(ctx, m.stepPin, false)
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute.
func (m *gpioStepper) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	if revolutions == 0 {
		return errors.New("revolutions can't be 0 for a stepper motor")
	}

	if d == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		revolutions *= -1
	}

	if d == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED || rpm == 0 {
		return m.Off(ctx)
	}

	if rpm < 0 {
		revolutions *= -1
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	if !m.threadStarted {
		return errors.New("thread not started")
	}

	m.targetStepPosition += int64(revolutions * float64(m.stepsPerRotation))
	m.targetStepsPerSecond = int64(rpm * float64(m.stepsPerRotation) / 60.0)
	if m.targetStepsPerSecond == 0 {
		m.targetStepsPerSecond = 1
	}

	return nil
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero), at a specific speed.
func (m *gpioStepper) GoTo(ctx context.Context, rpm float64, position float64) error {
	curPos, err := m.Position(ctx)
	if err != nil {
		return err
	}
	return m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, rpm, position-curPos)
}

// GoTillStop moves a motor until stopped. The "stop" mechanism is up to the underlying motor implementation.
// Ex: EncodedMotor goes until physically stopped/stalled (detected by change in position being very small over a fixed time.)
// Ex: TMCStepperMotor has "StallGuard" which detects the current increase when obstructed and stops when that reaches a threshold.
// Ex: Other motors may use an endstop switch (such as via a DigitalInterrupt) or be configured with other sensors.
func (m *gpioStepper) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("gpioStepper GoTillStop not done yet")
}

// Set the current position (+/- offset) to be the new zero (home) position.
func (m *gpioStepper) Zero(ctx context.Context, offset float64) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.stepPosition = int64(offset * float64(m.stepsPerRotation))
	return nil
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *gpioStepper) Position(ctx context.Context) (float64, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return float64(m.stepPosition) / float64(m.stepsPerRotation), nil
}

// PositionSupported returns whether or not the motor supports reporting of its position which
// is reliant on having an encoder.
func (m *gpioStepper) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// Off turns the motor off.
func (m *gpioStepper) Off(ctx context.Context) error {
	m.stop()
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.enable(ctx, false)
}

func (m *gpioStepper) stop() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.targetStepPosition = m.stepPosition
	m.targetStepsPerSecond = 0
}

// IsOn returns whether or not the motor is currently on.
func (m *gpioStepper) IsOn(ctx context.Context) (bool, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.stepPosition != m.targetStepPosition, nil
}

func (m *gpioStepper) enable(ctx context.Context, on bool) error {
	if m.enablePinHigh != "" {
		return m.theBoard.GPIOSet(ctx, m.enablePinHigh, on)
	}

	if m.enablePinLow != "" {
		return m.theBoard.GPIOSet(ctx, m.enablePinLow, !on)
	}

	return nil
}
