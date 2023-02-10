// Package unipolarfivewirestepper implements a GPIO based
// stepper motor (model: 28byj-48) with uln2003 controler.
package unipolarfivewirestepper

/*
	Motor Name:  		28byj-48
	Motor Controler: 	ULN2003
	Datasheet:
			ULN2003: 	https://www.makerguides.com/wp-content/uploads/2019/04/ULN2003-Datasheet.pdf
			28byj-48:	https://components101.com/sites/default/files/component_datasheet/28byj48-step-motor-datasheet.pdf

	This driver will drive the motor with half-step driving method (instead of full-step drive) for higher resolutions.
	In half-step the current vector divides a circle into eight parts. The eight step switching sequence is shown in
	stepSequence below. The motor takes 5.625*(1/64)° per step. For 360° the motor will take 4096 steps.

	We set the minimum sleep time between steps to be 0.002s to prevent hardware damage. The motor also has a max speed
	of 10-15 rpm at 5V.
*/

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var (
	model                = resource.NewDefaultModel("28byj48")
	minStepDelay         = 0.002                  // minimum sleep time between steps
	minDelayBetweenTicks = 100 * time.Microsecond // minimum sleep time between each ticks
)

// stepSequence contains switching signal for uln2003 pins.
// Treversing through stepSequence once is one step.
var stepSequence = [8][4]bool{
	{true, false, false, true},
	{true, false, false, false},
	{true, true, false, false},
	{false, true, false, false},
	{false, true, true, false},
	{false, false, true, false},
	{false, false, true, true},
	{false, false, false, true},
}

// PinConfig defines the mapping of where motor are wired.
type PinConfig struct {
	In1 string `json:"In1"`
	In2 string `json:"In2"`
	In3 string `json:"In3"`
	In4 string `json:"In4"`
}

// Config describes the configuration of a motor.
type Config struct {
	Pins             PinConfig `json:"pins"`
	BoardName        string    `json:"board"`
	TicksPerRotation int       `json:"ticks_per_rotation"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) ([]string, error) {
	var deps []string
	if config.BoardName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	if config.Pins.In1 == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "In1")
	}

	if config.Pins.In2 == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "In2")
	}

	if config.Pins.In3 == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "In3")
	}

	if config.Pins.In4 == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "In4")
	}

	deps = append(deps, config.BoardName)
	return deps, nil
}

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return new28byj(deps, config, config.Name, logger)
		},
	}
	registry.RegisterComponent(motor.Subtype, model, _motor)
	config.RegisterComponentAttributeMapConverter(
		motor.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{},
	)
}

func new28byj(deps registry.Dependencies, config config.Component, name string, logger golog.Logger) (motor.Motor, error) {
	mc, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(mc, config.ConvertedAttributes)
	}

	b, err := board.FromDependencies(deps, mc.BoardName)
	if err != nil {
		return nil, errors.Wrap(err, " expected board name in config for motor")
	}

	if mc.TicksPerRotation <= 0 {
		return nil, errors.New("expected ticks_per_rotation to be greater than zero in config for motor")
	}

	m := &uln28byj{
		theBoard:         b,
		ticksPerRotation: mc.TicksPerRotation,
		logger:           logger,
		motorName:        name,
	}

	if mc.Pins.In1 != "" {
		in1, err := b.GPIOPinByName(mc.Pins.In1)
		if err != nil {
			return nil, errors.Wrapf(err, " in In1 in motor (%s)", m.motorName)
		}
		m.in1 = in1
	}

	if mc.Pins.In2 != "" {
		in2, err := b.GPIOPinByName(mc.Pins.In2)
		if err != nil {
			return nil, errors.Wrapf(err, " in In2 in motor (%s)", m.motorName)
		}
		m.in2 = in2
	}

	if mc.Pins.In3 != "" {
		in3, err := b.GPIOPinByName(mc.Pins.In3)
		if err != nil {
			return nil, errors.Wrapf(err, " in In3 in motor (%s)", m.motorName)
		}
		m.in3 = in3
	}

	if mc.Pins.In4 != "" {
		in4, err := b.GPIOPinByName(mc.Pins.In4)
		if err != nil {
			return nil, errors.Wrapf(err, " in In4 in motor (%s)", m.motorName)
		}
		m.in4 = in4
	}

	return m, nil
}

// struct is named after the controler uln28byj.
type uln28byj struct {
	theBoard           board.Board
	ticksPerRotation   int
	in1, in2, in3, in4 board.GPIOPin
	logger             golog.Logger
	motorName          string

	// state
	lock  sync.Mutex
	opMgr operation.SingleOperationManager

	stepPosition         int64
	stepperDelay         float64
	targetStepPosition   int64
	targetStepsPerSecond int64
	generic.Unimplemented
}

// doRun runs the motor till it reaches target step position.
func (m *uln28byj) doRun(ctx context.Context) {
	for {
		m.lock.Lock()
		if m.stepPosition == m.targetStepPosition {
			err := m.enable(ctx, false)
			if err != nil {
				m.logger.Info("error while enabling motor")
			}
			m.lock.Unlock()
			break
		}

		err := m.doStep(ctx, m.stepPosition < m.targetStepPosition)
		m.lock.Unlock()
		if err != nil {
			m.logger.Info("error stepping %w", err)
			break
		}
	}
}

// doStep has to be locked to call.
// Depending on the direction, doStep will either treverse the stepSequence array in ascending
// or descending order.
func (m *uln28byj) doStep(ctx context.Context, forward bool) error {
	if forward {
		for tick := 0; tick < len(stepSequence); tick++ {
			err := m.doTicks(ctx, tick)
			if err != nil {
				return err
			}
		}
		time.Sleep(time.Duration(m.stepperDelay))
		m.stepPosition++
	} else {
		for tick := len(stepSequence) - 1; tick >= 0; tick-- {
			err := m.doTicks(ctx, tick)
			if err != nil {
				return err
			}
		}
		time.Sleep(time.Duration(m.stepperDelay))
		m.stepPosition--
	}

	return nil
}

// doTicks sets all 4 pins.
func (m *uln28byj) doTicks(ctx context.Context, tick int) error {
	err1 := m.in1.Set(ctx, stepSequence[tick][0], nil)
	if err1 != nil {
		return errors.Errorf("failed to set In1 with error in motor (%s)", m.motorName)
	}

	time.Sleep(minDelayBetweenTicks)
	err2 := m.in2.Set(ctx, stepSequence[tick][1], nil)
	if err2 != nil {
		return errors.Errorf("failed to set In2 with error in motor (%s)", m.motorName)
	}

	time.Sleep(minDelayBetweenTicks)

	err3 := m.in3.Set(ctx, stepSequence[tick][2], nil)
	if err3 != nil {
		return errors.Errorf("failed to set In3 with error in motor (%s)", m.motorName)
	}

	time.Sleep(minDelayBetweenTicks)

	err4 := m.in4.Set(ctx, stepSequence[tick][3], nil)
	if err4 != nil {
		return errors.Errorf("failed to set In4 with error in motor (%s)", m.motorName)
	}
	return nil
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
// can be assigned negative values to move in a backwards direction. Note: if both are negative
// the motor will spin in the forward direction.
func (m *uln28byj) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	if rpm == 0 {
		return motor.NewZeroRPMError()
	}

	ctx, done := m.opMgr.New(ctx)
	defer done()

	if math.Abs(rpm) < 0.1 {
		m.logger.Info("RPM is less than 0.1 threshold, stopping motor ")
		return m.Stop(ctx, nil)
	}

	if revolutions == 0 {
		revolutions = 1000000.0
	}
	var d int64 = 1

	if math.Signbit(revolutions) != math.Signbit(rpm) {
		d = -1
	}

	revolutions = math.Abs(revolutions)
	rpm = math.Abs(rpm) * float64(d)

	m.lock.Lock()

	m.targetStepPosition += int64(float64(d)*revolutions*float64(m.ticksPerRotation)) / 8
	m.targetStepsPerSecond = int64(revolutions * float64(m.ticksPerRotation) / 60.0)
	if m.targetStepsPerSecond == 0 {
		m.targetStepsPerSecond = 1
	}

	// stepperDelay is the wait time between each step taken.
	// The minimum value is set to 0.002s, anything less then this can potentially damage the gears.
	m.stepperDelay = math.Max(1/((rpm*8)/60), minStepDelay)

	m.lock.Unlock()

	m.doRun(ctx)
	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop)
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific RPM. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target.
func (m *uln28byj) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	curPos, err := m.Position(ctx, extra)
	if err != nil {
		return errors.Wrapf(err, "error in GoTo from motor (%s)", m.motorName)
	}
	moveDistance := positionRevolutions - (curPos/float64(m.ticksPerRotation))/8

	return m.GoFor(ctx, math.Abs(rpm), moveDistance, extra)
}

// Set the current position (+/- offset) to be the new zero (home) position.
func (m *uln28byj) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	return motor.NewResetZeroPositionUnsupportedError(m.motorName)
}

// SetPower is invalid for this motor.
func (m *uln28byj) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	return errors.Errorf("doesn't support raw power mode in motor (%s)", m.motorName)
}

// Position reports the current step position of the motor. If it's not supported, the returned
// data is undefined.
func (m *uln28byj) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return float64(m.stepPosition), nil
}

// Properties returns the status of whether the motor supports certain optional features.
func (m *uln28byj) Properties(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

// IsMoving returns if the motor is currently moving.
func (m *uln28byj) IsMoving(ctx context.Context) (bool, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.stepPosition != m.targetStepPosition, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *uln28byj) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.stop()
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.enable(ctx, false)
}

func (m *uln28byj) stop() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.targetStepPosition = m.stepPosition
	m.targetStepsPerSecond = 0
}

// IsPowered returns whether or not the motor is currently on. It also returns the percent power
// that the motor has, but stepper motors only have this set to 0% or 100%, so it's a little
// redundant.
func (m *uln28byj) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	on, err := m.IsMoving(ctx)
	if err != nil {
		return on, 0.0, errors.Wrapf(err, "error in IsPowered from motor (%s)", m.motorName)
	}
	percent := 0.0
	if on {
		percent = 1.0
	}
	return on, percent, err
}

// enable sets pin value to true or false. If all pin set to false, motor will be idle.
func (m *uln28byj) enable(ctx context.Context, on bool) error {
	err := multierr.Combine(
		m.in1.Set(ctx, on, nil),
		m.in2.Set(ctx, on, nil),
		m.in3.Set(ctx, on, nil),
		m.in4.Set(ctx, on, nil),
	)

	return err
}
