// Package gpiostepper implements a GPIO based stepper motor
package gpiostepper

// This package is meant to be used with bipolar stepper motors connected to drivers that drive motors
// using high/low direction pins and pulses to step the motor armatures, the package can also set enable
// pins high or low if the driver needs them to power the stepper motor armatures
/*
   Compatibility tested:
   Stepper Motors:     NEMA
   Motor Driver:   DRV8825, A4998, L298N igus-drylin D8(X)
   Resources:
           DRV8825:    https://lastminuteengineers.com/drv8825-stepper-motor-driver-arduino-tutorial/
           A4998:  https://lastminuteengineers.com/a4988-stepper-motor-driver-arduino-tutorial/
           L298N: https://lastminuteengineers.com/stepper-motor-l298n-arduino-tutorial/

   This driver will drive the motor using a step pulse with a delay that matches the speed calculated by:
   stepperDelay (ns) := 1min / (rpm (revs_per_minute) * spr (steps_per_revolution))
   The motor will then step and increment its position until it has reached a target or has been stopped.

   Configuration:
   Required pins: a step pin to send pulses and a direction pin to set the direction.
   Enabling current to flow through the armature and holding a position can be done by setting enable pins on
   hardware that supports that functionality.

   An optional configurable stepper_delay parameter configures the minimum delay to set a pulse to high
   for a particular stepper motor. This is usually motor specific and can be calculated using phase
   resistance and induction data from the datasheet of your stepper motor.
*/

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

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("gpiostepper")

// PinConfig defines the mapping of where motor are wired.
type PinConfig struct {
	Step          string `json:"step"`
	Direction     string `json:"dir"`
	EnablePinHigh string `json:"en_high,omitempty"`
	EnablePinLow  string `json:"en_low,omitempty"`
}

// Config describes the configuration of a motor.
type Config struct {
	Pins             PinConfig `json:"pins"`
	BoardName        string    `json:"board"`
	StepperDelay     int       `json:"stepper_delay_usec,omitempty"` // When using stepper motors, the time to remain high
	TicksPerRotation int       `json:"ticks_per_rotation"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.BoardName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	if cfg.TicksPerRotation == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "ticks_per_rotation")
	}
	if cfg.Pins.Direction == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "dir")
	}
	if cfg.Pins.Step == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "step")
	}
	deps = append(deps, cfg.BoardName)
	return deps, nil
}

func init() {
	resource.RegisterComponent(motor.API, model, resource.Registration[motor.Motor, *Config]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (motor.Motor, error) {
			actualBoard, motorConfig, err := getBoardFromRobotConfig(deps, conf)
			if err != nil {
				return nil, err
			}

			return newGPIOStepper(ctx, actualBoard, *motorConfig, conf.ResourceName(), logger)
		},
	})
}

func getBoardFromRobotConfig(deps resource.Dependencies, conf resource.Config) (board.Board, *Config, error) {
	motorConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, nil, err
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

func newGPIOStepper(
	ctx context.Context,
	b board.Board,
	mc Config,
	name resource.Name,
	logger golog.Logger,
) (motor.Motor, error) {
	if b == nil {
		return nil, errors.New("board is required")
	}

	if mc.TicksPerRotation == 0 {
		return nil, errors.New("expected ticks_per_rotation in config for motor")
	}

	m := &gpioStepper{
		Named:            name.AsNamed(),
		theBoard:         b,
		stepsPerRotation: mc.TicksPerRotation,
		logger:           logger,
		opMgr:            operation.NewSingleOperationManager(),
	}

	var err error

	// only set enable pins if they exist
	if mc.Pins.EnablePinHigh != "" {
		m.enablePinHigh, err = b.GPIOPinByName(mc.Pins.EnablePinHigh)
		if err != nil {
			return nil, err
		}
	}
	if mc.Pins.EnablePinLow != "" {
		m.enablePinLow, err = b.GPIOPinByName(mc.Pins.EnablePinLow)
		if err != nil {
			return nil, err
		}
	}

	// set the required step and direction pins
	m.stepPin, err = b.GPIOPinByName(mc.Pins.Step)
	if err != nil {
		return nil, err
	}

	m.dirPin, err = b.GPIOPinByName(mc.Pins.Direction)
	if err != nil {
		return nil, err
	}

	if mc.StepperDelay > 0 {
		m.minDelay = time.Duration(mc.StepperDelay * int(time.Microsecond))
	}

	err = m.enable(ctx, false)
	if err != nil {
		return nil, err
	}

	m.startThread()
	return m, nil
}

type gpioStepper struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	// config
	theBoard                    board.Board
	stepsPerRotation            int
	stepperDelay                time.Duration
	minDelay                    time.Duration
	enablePinHigh, enablePinLow board.GPIOPin
	stepPin, dirPin             board.GPIOPin
	logger                      golog.Logger

	// state
	lock  sync.Mutex
	opMgr *operation.SingleOperationManager

	stepPosition       int64
	threadStarted      bool
	targetStepPosition int64

	cancel    context.CancelFunc
	waitGroup sync.WaitGroup
}

// SetPower sets the percentage of power the motor should employ between 0-1.
func (m *gpioStepper) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	if math.Abs(powerPct) <= .0001 {
		m.stop()
		return nil
	}

	return errors.Errorf("gpioStepper doesn't support raw power mode in motor (%s)", m.Name().Name)
}

func (m *gpioStepper) startThread() {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.threadStarted {
		return
	}

	m.logger.Debugf("starting control thread for motor (%s)", m.Name().Name)

	var ctxWG context.Context
	ctxWG, m.cancel = context.WithCancel(context.Background())
	m.threadStarted = true
	m.waitGroup.Add(1)
	go func() {
		defer m.waitGroup.Done()
		for {
			sleep, err := m.doCycle(ctxWG)
			if err != nil {
				m.logger.Warnf("error cycling gpioStepper (%s) %s", m.Name().Name, err.Error())
			}

			if !utils.SelectContextOrWait(ctxWG, sleep) {
				// context done
				return
			}
		}
	}()
}

func (m *gpioStepper) doCycle(ctx context.Context) (time.Duration, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// thread waits until something changes the target position in the
	// gpiostepper struct
	if m.stepPosition == m.targetStepPosition {
		return 5 * time.Millisecond, nil
	}

	// TODO: Setting PWM here works much better than steps to set speed
	// Redo this part with PWM logic, but also be aware that parallel
	// logic to the PWM call will need to be implemented to account for position
	// reporting
	err := m.doStep(ctx, m.stepPosition < m.targetStepPosition)
	if err != nil {
		return time.Second, fmt.Errorf("error stepping motor (%s) %w", m.Name().Name, err)
	}

	// wait the stepper delay to return from the doRun for loop or select
	// context if the duration has not elapsed.
	return m.stepperDelay, nil
}

// have to be locked to call.
func (m *gpioStepper) doStep(ctx context.Context, forward bool) error {
	err := multierr.Combine(
		m.dirPin.Set(ctx, forward, nil),
		m.stepPin.Set(ctx, true, nil))
	if err != nil {
		return err
	}
	// stay high for half the delay
	time.Sleep(m.stepperDelay / 2)

	if err := m.stepPin.Set(ctx, false, nil); err != nil {
		return err
	}

	// stay low for the other half
	time.Sleep(m.stepperDelay / 2)

	if forward {
		m.stepPosition++
	} else {
		m.stepPosition--
	}

	return nil
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
// can be assigned negative values to move in a backwards direction. Note: if both are negative
// the motor will spin in the forward direction.
func (m *gpioStepper) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	err := m.enable(ctx, true)
	if err != nil {
		return errors.Wrapf(err, "error enabling motor in GoFor from motor (%s)", m.Name().Name)
	}

	err = m.goForInternal(ctx, rpm, revolutions)
	if err != nil {
		return multierr.Combine(
			m.enable(ctx, false),
			errors.Wrapf(err, "error in GoFor from motor (%s)", m.Name().Name))
	}

	// this is a long-running operation, do not wait for Stop, do not disable enable pins
	if revolutions == 0 {
		return nil
	}

	return multierr.Combine(
		m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop),
		m.enable(ctx, false))
}

func (m *gpioStepper) goForInternal(ctx context.Context, rpm, revolutions float64) error {
	if revolutions == 0 {
		// go a large number of revolutions if 0 is passed in, at the desired speed
		revolutions = 1000000
	}

	speed := math.Abs(rpm)
	if speed < 0.1 {
		m.logger.Warn("motor speed is nearly 0 rev_per_min")
		return m.Stop(ctx, nil)
	}

	var d int64 = 1
	if math.Signbit(revolutions) != math.Signbit(rpm) {
		d = -1
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	// calculate delay between steps for the thread in the gorootuine that we started in component creation
	m.stepperDelay = time.Duration(int64(float64(time.Minute) / (math.Abs(rpm) * float64(m.stepsPerRotation))))
	if m.stepperDelay < m.minDelay {
		m.stepperDelay = m.minDelay
		m.logger.Debugf(
			"calculated delay less than the minimum delay for stepper motor setting to %+v", m.stepperDelay,
		)
	}

	if !m.threadStarted {
		return errors.New("thread not started")
	}

	m.targetStepPosition += d * int64(math.Abs(revolutions)*float64(m.stepsPerRotation))

	return nil
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific RPM. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target.
func (m *gpioStepper) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	curPos, err := m.Position(ctx, extra)
	if err != nil {
		return errors.Wrapf(err, "error in GoTo from motor (%s)", m.Name().Name)
	}
	moveDistance := positionRevolutions - curPos

	// if you call GoFor with 0 revolutions, the motor will spin forever. If we are at the target,
	// we must avoid this by not calling GoFor.
	if rdkutils.Float64AlmostEqual(moveDistance, 0, 0.1) {
		m.logger.Debugf("GoTo distance nearly zero for motor (%s), not moving", m.Name().Name)
		return nil
	}

	m.logger.Debugf("motor (%s) going to %.2f at rpm %.2f", m.Name().Name, moveDistance, math.Abs(rpm))
	return m.GoFor(ctx, math.Abs(rpm), moveDistance, extra)
}

// Set the current position (+/- offset) to be the new zero (home) position.
func (m *gpioStepper) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.stepPosition = int64(-1 * offset * float64(m.stepsPerRotation))
	return nil
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *gpioStepper) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return float64(m.stepPosition) / float64(m.stepsPerRotation), nil
}

// Properties returns the status of whether the motor supports certain optional properties.
func (m *gpioStepper) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{
		PositionReporting: true,
	}, nil
}

// IsMoving returns if the motor is currently moving.
func (m *gpioStepper) IsMoving(ctx context.Context) (bool, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.stepPosition != m.targetStepPosition, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *gpioStepper) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.stop()
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.enable(ctx, false)
}

func (m *gpioStepper) stop() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.targetStepPosition = m.stepPosition
}

// IsPowered returns whether or not the motor is currently on. It also returns the percent power
// that the motor has, but stepper motors only have this set to 0% or 100%, so it's a little
// redundant.
func (m *gpioStepper) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	on, err := m.IsMoving(ctx)
	if err != nil {
		return on, 0.0, errors.Wrapf(err, "error in IsPowered from motor (%s)", m.Name().Name)
	}
	percent := 0.0
	if on {
		percent = 1.0
	}
	return on, percent, err
}

func (m *gpioStepper) enable(ctx context.Context, on bool) error {
	var err error
	if m.enablePinHigh != nil {
		err = multierr.Combine(err, m.enablePinHigh.Set(ctx, on, nil))
	}

	if m.enablePinLow != nil {
		err = multierr.Combine(err, m.enablePinLow.Set(ctx, !on, nil))
	}

	return err
}

func (m *gpioStepper) Close(ctx context.Context) error {
	err := m.Stop(ctx, nil)

	m.lock.Lock()
	defer m.lock.Unlock()
	if m.cancel != nil {
		m.logger.Debugf("stopping control thread for motor (%s)", m.Name().Name)
		m.cancel()
		m.cancel = nil
		m.waitGroup.Wait()
	}

	return err
}
