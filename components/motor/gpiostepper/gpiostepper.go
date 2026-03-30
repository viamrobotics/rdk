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

   This driver uses hardware PWM on the step pin to generate precise step pulses at the desired
   frequency. A 1kHz tracking goroutine estimates position based on elapsed time and the confirmed
   PWM frequency.

   Configuration:
   Required pins: a step pin to send pulses and a direction pin to set the direction.
   Enabling current to flow through the armature and holding a position can be done by setting enable pins on
   hardware that supports that functionality.

   An optional configurable stepper_delay parameter configures the minimum delay between pulses
   for a particular stepper motor. This sets the maximum step frequency (1/stepper_delay).
*/

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
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
func (cfg *Config) Validate(path string) ([]string, []string, error) {
	var deps []string
	if cfg.BoardName == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "board")
	}
	if cfg.TicksPerRotation == 0 {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "ticks_per_rotation")
	}
	if cfg.Pins.Direction == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "dir")
	}
	if cfg.Pins.Step == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "step")
	}
	deps = append(deps, cfg.BoardName)
	return deps, nil, nil
}

func init() {
	resource.RegisterComponent(motor.API, model, resource.Registration[motor.Motor, *Config]{
		Constructor: newGPIOStepper,
	},
	)
}

func (m *gpioStepper) enable(ctx context.Context, high bool) error {
	var err error
	if m.enablePinHigh != nil {
		err = multierr.Combine(err, m.enablePinHigh.Set(ctx, high, nil))
	}

	if m.enablePinLow != nil {
		err = multierr.Combine(err, m.enablePinLow.Set(ctx, !high, nil))
	}

	return err
}

func newGPIOStepper(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (motor.Motor, error) {
	mc, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	b, err := board.FromProvider(deps, mc.BoardName)
	if err != nil {
		return nil, err
	}

	if mc.TicksPerRotation == 0 {
		return nil, errors.New("expected ticks_per_rotation in config for motor")
	}

	m := &gpioStepper{
		Named:            conf.ResourceName().AsNamed(),
		theBoard:         b,
		stepsPerRotation: mc.TicksPerRotation,
		logger:           logger,
		opMgr:            operation.NewSingleOperationManager(),
	}

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

	return m, nil
}

type gpioStepper struct {
	resource.Named
	resource.AlwaysRebuild

	// config
	theBoard                    board.Board
	stepsPerRotation            int
	minDelay                    time.Duration
	enablePinHigh, enablePinLow board.GPIOPin
	stepPin, dirPin             board.GPIOPin
	logger                      logging.Logger

	// state
	lock  sync.Mutex
	opMgr *operation.SingleOperationManager

	stepPosition       atomic.Int64
	targetStepPosition atomic.Int64
	trackingCancel     context.CancelFunc
}

// rpmToFreqHz converts RPM to step frequency in Hz, clamped by minDelay.
func (m *gpioStepper) rpmToFreqHz(rpm float64) uint {
	freq := math.Abs(rpm) * float64(m.stepsPerRotation) / 60.0
	if m.minDelay > 0 {
		maxFreq := 1.0 / m.minDelay.Seconds()
		if freq > maxFreq {
			freq = maxFreq
		}
	}
	if freq < 1 {
		freq = 1
	}
	return uint(math.Round(freq))
}

// startPWM sets direction, enables the motor, and starts PWM on the step pin.
// Must be called under m.lock.
func (m *gpioStepper) startPWM(ctx context.Context, forward bool, freqHz uint) (float64, error) {
	if err := m.dirPin.Set(ctx, forward, nil); err != nil {
		return 0, fmt.Errorf("error setting direction pin: %w", err)
	}
	if err := m.enable(ctx, true); err != nil {
		return 0, fmt.Errorf("error enabling motor: %w", err)
	}
	if err := m.stepPin.SetPWMFreq(ctx, freqHz, nil); err != nil {
		return 0, fmt.Errorf("error setting PWM frequency: %w", err)
	}
	if err := m.stepPin.SetPWM(ctx, 0.5, nil); err != nil {
		return 0, fmt.Errorf("error setting PWM duty cycle: %w", err)
	}

	// Read back confirmed frequency. Non-fatal: fall back to requested freq on error.
	confirmedFreq, err := m.stepPin.PWMFreq(ctx, nil)
	if err != nil {
		m.logger.Infof("PWM freq requested %d Hz, readback failed: %v; using requested value", freqHz, err)
		return float64(freqHz), nil
	}

	actualFreq := float64(confirmedFreq)
	if actualFreq == 0 {
		actualFreq = float64(freqHz)
	}

	return actualFreq, nil
}

// stopHardware stops PWM and disables the motor. Idempotent.
func (m *gpioStepper) stopHardware(ctx context.Context) error {
	return multierr.Combine(
		m.stepPin.SetPWM(ctx, 0, nil),
		m.enable(ctx, false),
	)
}

// trackPosition is a per-movement goroutine that estimates position based on elapsed time
// and the confirmed PWM frequency.
func (m *gpioStepper) trackPosition(ctx context.Context, doneCh chan<- error,
	targetSteps int64, forward bool, actualFreqHz float64,
) {
	var result error
	defer func() {
		if ctx.Err() == nil {
			// Natural exit (target reached) — we own hardware cleanup
			if err := m.stopHardware(context.Background()); err != nil {
				m.logger.Warnf("error stopping hardware after motion complete: %v", err)
			}
		}
		// If cancelled, hardware is managed by the caller (new movement or Stop)
		if doneCh != nil {
			doneCh <- result
		}
	}()

	ticker := time.NewTicker(time.Millisecond) // 1kHz
	defer ticker.Stop()
	lastTime := time.Now()
	var accumulator float64
	indefinite := targetSteps == math.MaxInt64 || targetSteps == math.MinInt64

	for {
		select {
		case <-ctx.Done():
			result = errors.New("trackPosition: context cancelled")
			return
		case now := <-ticker.C:
			elapsed := now.Sub(lastTime)
			lastTime = now
			accumulator += elapsed.Seconds() * actualFreqHz
			wholeSteps := int64(accumulator)
			accumulator -= float64(wholeSteps)

			curPos := m.stepPosition.Load()
			if forward {
				curPos += wholeSteps
			} else {
				curPos -= wholeSteps
			}

			if !indefinite {
				if (forward && curPos >= targetSteps) || (!forward && curPos <= targetSteps) {
					m.stepPosition.Store(targetSteps)
					result = nil
					return
				}
			}
			m.stepPosition.Store(curPos)
		}
	}
}

// SetPower sets the percentage of power the motor should employ between 0-1.
func (m *gpioStepper) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	if math.Abs(powerPct) <= .0001 {
		return m.Stop(ctx, nil)
	}

	if m.minDelay == 0 {
		return errors.Errorf(
			"if you want to set the power, set 'stepper_delay_usec' in the motor config at "+
				"the minimum time delay between pulses for your stepper motor (%s)",
			m.Name().Name)
	}

	m.opMgr.CancelRunning(ctx)

	forward := powerPct > 0
	freqHz := uint(math.Abs(powerPct) / m.minDelay.Seconds())
	freqHz = max(1, freqHz)

	var target int64
	if forward {
		target = math.MaxInt64
	} else {
		target = math.MinInt64
	}
	m.targetStepPosition.Store(target)

	m.lock.Lock()
	if m.trackingCancel != nil {
		m.trackingCancel()
	}
	actualFreq, err := m.startPWM(ctx, forward, freqHz)
	if err != nil {
		m.lock.Unlock()
		return err
	}
	trackCtx, cancel := context.WithCancel(context.Background())
	m.trackingCancel = cancel
	m.lock.Unlock()

	utils.PanicCapturingGo(func() { m.trackPosition(trackCtx, nil, target, forward, actualFreq) })
	return nil
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
// can be assigned negative values to move in a backwards direction. Note: if both are negative
// the motor will spin in the forward direction.
func (m *gpioStepper) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	speed := math.Abs(rpm)
	if speed < 0.1 {
		return multierr.Combine(m.Stop(ctx, nil), motor.NewZeroRPMError())
	}

	if err := motor.CheckRevolutions(revolutions); err != nil {
		return err
	}

	var d int64 = 1
	if math.Signbit(revolutions) != math.Signbit(rpm) {
		d = -1
	}

	forward := d > 0
	target := m.stepPosition.Load() + d*int64(math.Abs(revolutions)*float64(m.stepsPerRotation))
	m.targetStepPosition.Store(target)

	m.lock.Lock()
	if m.trackingCancel != nil {
		m.trackingCancel()
	}
	actualFreq, err := m.startPWM(ctx, forward, m.rpmToFreqHz(rpm))
	if err != nil {
		m.lock.Unlock()
		return err
	}
	trackCtx, cancel := context.WithCancel(ctx)
	m.trackingCancel = cancel
	m.lock.Unlock()

	doneCh := make(chan error, 1)
	utils.PanicCapturingGo(func() { m.trackPosition(trackCtx, doneCh, target, forward, actualFreq) })
	err = <-doneCh
	if ctx.Err() != nil {
		// Context was cancelled (external cancel or opMgr interrupt) — clean up
		m.targetStepPosition.Store(m.stepPosition.Load())
		if hwErr := m.stopHardware(context.Background()); hwErr != nil {
			err = multierr.Combine(err, hwErr)
		}
	}
	return err
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
		m.logger.CDebugf(ctx, "GoTo distance nearly zero for motor (%s), not moving", m.Name().Name)
		return nil
	}

	m.logger.CDebugf(ctx, "motor (%s) going to %.2f at rpm %.2f", m.Name().Name, moveDistance, math.Abs(rpm))
	return m.GoFor(ctx, math.Abs(rpm), moveDistance, extra)
}

// SetRPM instructs the motor to move at the specified RPM indefinitely.
func (m *gpioStepper) SetRPM(ctx context.Context, rpm float64, extra map[string]interface{}) error {
	if math.Abs(rpm) <= .0001 {
		return m.Stop(ctx, nil)
	}

	m.opMgr.CancelRunning(ctx)

	forward := rpm > 0
	var target int64
	if forward {
		target = math.MaxInt64
	} else {
		target = math.MinInt64
	}
	m.targetStepPosition.Store(target)

	m.lock.Lock()
	if m.trackingCancel != nil {
		m.trackingCancel()
	}
	actualFreq, err := m.startPWM(ctx, forward, m.rpmToFreqHz(rpm))
	if err != nil {
		m.lock.Unlock()
		return err
	}
	trackCtx, cancel := context.WithCancel(context.Background())
	m.trackingCancel = cancel
	m.lock.Unlock()

	utils.PanicCapturingGo(func() { m.trackPosition(trackCtx, nil, target, forward, actualFreq) })
	return nil
}

// Set the current position (+/- offset) to be the new zero (home) position.
func (m *gpioStepper) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	m.lock.Lock()
	if m.trackingCancel != nil {
		m.trackingCancel()
	}
	m.lock.Unlock()
	// stopHardware is needed because the cancelled tracking goroutine skips it.
	if err := m.stopHardware(ctx); err != nil {
		return err
	}
	pos := int64(-1 * offset * float64(m.stepsPerRotation))
	m.stepPosition.Store(pos)
	m.targetStepPosition.Store(pos)
	return nil
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *gpioStepper) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return float64(m.stepPosition.Load()) / float64(m.stepsPerRotation), nil
}

// Properties returns the status of whether the motor supports certain optional properties.
func (m *gpioStepper) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{
		PositionReporting: true,
	}, nil
}

// IsMoving returns if the motor is currently moving.
func (m *gpioStepper) IsMoving(ctx context.Context) (bool, error) {
	return m.stepPosition.Load() != m.targetStepPosition.Load(), nil
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *gpioStepper) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.lock.Lock()
	if m.trackingCancel != nil {
		m.trackingCancel()
	}
	m.lock.Unlock()
	m.targetStepPosition.Store(m.stepPosition.Load())
	return m.stopHardware(ctx)
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

func (m *gpioStepper) Close(ctx context.Context) error {
	m.lock.Lock()
	if m.trackingCancel != nil {
		m.trackingCancel()
	}
	m.lock.Unlock()
	return m.stopHardware(ctx)
}
