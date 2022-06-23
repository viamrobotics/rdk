// Package gpiostepper implements a GPIO based stepper motor.
package gpiostepper

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

const modelName = "gpiostepper"

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			actualBoard, motorConfig, err := getBoardFromRobotConfig(deps, config)
			if err != nil {
				return nil, err
			}

			return newGPIOStepper(ctx, actualBoard, *motorConfig, logger)
		},
	}
	registry.RegisterComponent(motor.Subtype, modelName, _motor)
	motor.RegisterConfigAttributeConverter(modelName)
}

func getBoardFromRobotConfig(deps registry.Dependencies, config config.Component) (board.Board, *motor.Config, error) {
	motorConfig, ok := config.ConvertedAttributes.(*motor.Config)
	if !ok {
		return nil, nil, rdkutils.NewUnexpectedTypeError(motorConfig, config.ConvertedAttributes)
	}
	if motorConfig.BoardName == "" {
		return nil, nil, errors.New("expected board name in config for motor")
	}
	b, err := board.FromDependencies(deps, motorConfig.BoardName)
	if err != nil {
		return nil, nil, err
	}
	return b, motorConfig, nil
}

func newGPIOStepper(ctx context.Context, b board.Board, mc motor.Config, logger golog.Logger) (motor.Motor, error) {
	m := &gpioStepper{
		theBoard:         b,
		stepsPerRotation: mc.TicksPerRotation,
		stepperDelay:     mc.StepperDelay,
		logger:           logger,
	}

	if mc.Pins.EnablePinHigh != "" {
		enablePinHigh, err := b.GPIOPinByName(mc.Pins.EnablePinHigh)
		if err != nil {
			return nil, err
		}
		m.enablePinHigh = enablePinHigh
	}
	if mc.Pins.EnablePinLow != "" {
		enablePinLow, err := b.GPIOPinByName(mc.Pins.EnablePinLow)
		if err != nil {
			return nil, err
		}
		m.enablePinLow = enablePinLow
	}
	if mc.Pins.Step != "" {
		stepPin, err := b.GPIOPinByName(mc.Pins.Step)
		if err != nil {
			return nil, err
		}
		m.stepPin = stepPin
	}
	if mc.Pins.Direction != "" {
		directionPin, err := b.GPIOPinByName(mc.Pins.Direction)
		if err != nil {
			return nil, err
		}
		m.dirPin = directionPin
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	m.startThread(ctx)
	return m, nil
}

type gpioStepper struct {
	// config
	theBoard                    board.Board
	stepsPerRotation            int
	stepperDelay                uint
	enablePinHigh, enablePinLow board.GPIOPin
	stepPin, dirPin             board.GPIOPin
	logger                      golog.Logger

	// state
	lock  sync.Mutex
	opMgr operation.SingleOperationManager

	stepPosition         int64
	threadStarted        bool
	targetStepPosition   int64
	targetStepsPerSecond int64
	generic.Unimplemented
}

// validate if this config is valid.
func (m *gpioStepper) Validate() error {
	if m.theBoard == nil {
		return errors.New("need a board for gpioStepper")
	}

	if m.stepsPerRotation == 0 {
		m.stepsPerRotation = 200
	}

	if m.stepperDelay == 0 {
		m.stepperDelay = 20
	}

	if m.stepPin == nil {
		return errors.New("need a 'step' pin for gpioStepper")
	}

	if m.dirPin == nil {
		return errors.New("need a 'dir' pin for gpioStepper")
	}

	return nil
}

// SetPower sets the percentage of power the motor should employ between 0-1.
func (m *gpioStepper) SetPower(ctx context.Context, powerPct float64) error {
	if math.Abs(powerPct) <= .0001 {
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

	return time.Duration(int64(time.Microsecond*1000*1000) / int64(math.Abs(float64(m.targetStepsPerSecond)))), nil
}

// have to be locked to call.
func (m *gpioStepper) doStep(ctx context.Context, forward bool) error {
	err := multierr.Combine(
		m.enable(ctx, true),
		m.stepPin.Set(ctx, true),
		m.dirPin.Set(ctx, forward),
	)
	if err != nil {
		return err
	}

	time.Sleep(time.Duration(m.stepperDelay) * time.Microsecond)

	if forward {
		m.stepPosition++
	} else {
		m.stepPosition--
	}
	return m.stepPin.Set(ctx, false)
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
// can be assigned negative values to move in a backwards direction. Note: if both are negative
// the motor will spin in the forward direction.
func (m *gpioStepper) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	err := m.goForInternal(ctx, rpm, revolutions)
	if err != nil {
		return err
	}

	if revolutions == 0 {
		return nil
	}
	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m)
}

func (m *gpioStepper) goForInternal(ctx context.Context, rpm float64, revolutions float64) error {
	if revolutions == 0 {
		revolutions = 1000000.0
	}
	var d int64 = 1

	if math.Signbit(revolutions) != math.Signbit(rpm) {
		d = -1
	}

	revolutions = math.Abs(revolutions)
	rpm = math.Abs(rpm) * float64(d)

	if math.Abs(rpm) < 0.1 {
		return m.Stop(ctx)
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	if !m.threadStarted {
		return errors.New("thread not started")
	}

	m.targetStepPosition += int64(float64(d) * revolutions * float64(m.stepsPerRotation))
	m.targetStepsPerSecond = int64(rpm * float64(m.stepsPerRotation) / 60.0)
	if m.targetStepsPerSecond == 0 {
		m.targetStepsPerSecond = 1
	}

	return nil
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific RPM. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target.
func (m *gpioStepper) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	curPos, err := m.GetPosition(ctx)
	if err != nil {
		return err
	}
	moveDistance := positionRevolutions - curPos

	return m.GoFor(ctx, math.Abs(rpm), moveDistance)
}

// GoTillStop moves a motor until stopped. The "stop" mechanism is up to the underlying motor implementation.
// Ex: EncodedMotor goes until physically stopped/stalled (detected by change in position being very small over a fixed time.)
// Ex: TMCStepperMotor has "StallGuard" which detects the current increase when obstructed and stops when that reaches a threshold.
// Ex: Other motors may use an endstop switch (such as via a DigitalInterrupt) or be configured with other sensors.
func (m *gpioStepper) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	if err := m.GoFor(ctx, rpm, 0); err != nil {
		return err
	}
	defer func() {
		if err := m.Stop(ctx); err != nil {
			m.logger.Errorw("failed to turn off motor", "error", err)
		}
	}()
	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.Wrap(ctx.Err(), "stopped via context")
		}
		if stopFunc != nil && stopFunc(ctx) {
			return ctx.Err()
		}
	}
}

// Set the current position (+/- offset) to be the new zero (home) position.
func (m *gpioStepper) ResetZeroPosition(ctx context.Context, offset float64) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.stepPosition = int64(offset * float64(m.stepsPerRotation))
	return nil
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *gpioStepper) GetPosition(ctx context.Context) (float64, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return float64(m.stepPosition) / float64(m.stepsPerRotation), nil
}

// GetFeatures returns the status of whether the motor supports certain optional features.
func (m *gpioStepper) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

// IsMoving returns if the motor is currently on or off.
func (m *gpioStepper) IsMoving(ctx context.Context) (bool, error) {
	return m.IsPowered(ctx)
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *gpioStepper) Stop(ctx context.Context) error {
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

// IsPowered returns whether or not the motor is currently on.
func (m *gpioStepper) IsPowered(ctx context.Context) (bool, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.stepPosition != m.targetStepPosition, nil
}

func (m *gpioStepper) enable(ctx context.Context, on bool) error {
	if m.enablePinHigh != nil {
		return m.enablePinHigh.Set(ctx, on)
	}

	if m.enablePinLow != nil {
		return m.enablePinLow.Set(ctx, !on)
	}

	return nil
}
