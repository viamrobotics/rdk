// Package arduino implements the arduino board and some peripherals.
package arduino

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/motor/gpio"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// SetPowerZeroThreshold represents a power below which value attempting
// to run the motor simply stops it.
const SetPowerZeroThreshold = .0001

// init registers an arduino motor.
func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			motorConfig, ok := config.ConvertedAttributes.(*motor.Config)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(motorConfig, config.ConvertedAttributes)
			}
			if motorConfig.BoardName == "" {
				return nil, errors.New("expected board name in config for motor")
			}
			b, err := board.FromRobot(r, motorConfig.BoardName)
			if err != nil {
				return nil, err
			}
			// Note(erd): this would not be needed if encoders were a component
			actualBoard, ok := utils.UnwrapProxy(b).(*arduinoBoard)
			if !ok {
				return nil, errors.New("expected board to be an arduino board")
			}

			return configureMotorForBoard(ctx, actualBoard, config, motorConfig)
		},
	}

	registry.RegisterComponent(motor.Subtype, "arduino", _motor)

	motor.RegisterConfigAttributeConverter("arduino")
}

func configureMotorForBoard(
	ctx context.Context,
	b *arduinoBoard,
	config config.Component,
	motorConfig *motor.Config) (motor.MinimalMotor, error) {
	if !((motorConfig.Pins.PWM != "" && motorConfig.Pins.Direction != "") || (motorConfig.Pins.A != "" || motorConfig.Pins.B != "")) {
		return nil, errors.New("arduino needs at least a & b, or dir & pwm pins")
	}

	if motorConfig.EncoderA == "" || motorConfig.EncoderB == "" {
		return nil, errors.New("arduino needs a and b hall encoders")
	}

	if motorConfig.TicksPerRotation <= 0 {
		return nil, errors.New("arduino motors TicksPerRotation to be set")
	}

	if motorConfig.Pins.PWM == "" {
		motorConfig.Pins.PWM = "-1"
	}
	if motorConfig.Pins.A == "" {
		motorConfig.Pins.A = "-1"
	}
	if motorConfig.Pins.B == "" {
		motorConfig.Pins.B = "-1"
	}
	if motorConfig.Pins.Direction == "" {
		motorConfig.Pins.Direction = "-1"
	}
	if motorConfig.Pins.EnablePinLow == "" {
		motorConfig.Pins.EnablePinLow = "-1"
	}

	cmd := fmt.Sprintf("config-motor-dc %s %s %s %s %s %s e %s %s",
		config.Name,
		motorConfig.Pins.PWM,          // Optional if using A/B inputs (one of them will be PWMed if missing)
		motorConfig.Pins.A,            // Use either A & B, or DIR inputs, never both
		motorConfig.Pins.B,            // (A & B [& PWM] ) || (DIR & PWM)
		motorConfig.Pins.Direction,    // PWM is also required when using DIR
		motorConfig.Pins.EnablePinLow, // Always optional, inverting input (LOW = ENABLED)
		motorConfig.EncoderA,
		motorConfig.EncoderB,
	)

	res, err := b.runCommand(cmd)
	if err != nil {
		return nil, err
	}

	if res != "ok" {
		return nil, fmt.Errorf("got unknown response when configureMotor %s", res)
	}
	m, err := gpio.NewEncodedMotor(
		config,
		*motorConfig,
		&arduinoMotor{b, *motorConfig, config.Name},
		&encoder{b, *motorConfig, config.Name},
		b.logger,
	)
	if err != nil {
		return nil, err
	}
	if motorConfig.Pins.PWM != "-1" && motorConfig.PWMFreq > 0 {
		// When the motor controller has a PWM pin exposed (either (A && B && PWM) || (DIR && PWM))
		// We control the motor speed with the PWM pin
		err = b.SetPWMFreq(ctx, motorConfig.Pins.PWM, motorConfig.PWMFreq)
		if err != nil {
			return nil, err
		}
	} else if (motorConfig.Pins.A != "-1" && motorConfig.Pins.B != "-1") && motorConfig.PWMFreq > 0 {
		// When the motor controller only exposes A & B pin
		// We control the motor speed with both pins
		err = b.SetPWMFreq(ctx, motorConfig.Pins.A, motorConfig.PWMFreq)
		if err != nil {
			return nil, err
		}
		err = b.SetPWMFreq(ctx, motorConfig.Pins.B, motorConfig.PWMFreq)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

type arduinoMotor struct {
	b    *arduinoBoard
	cfg  motor.Config
	name string
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
func (m *arduinoMotor) SetPower(ctx context.Context, powerPct float64) error {
	if math.Abs(powerPct) <= SetPowerZeroThreshold {
		return m.Stop(ctx)
	}

	_, err := m.b.runCommand(fmt.Sprintf("motor-power %s %d", m.name, int(255.0*math.Abs(powerPct))))
	return err
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute.
func (m *arduinoMotor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	ticks := int(math.Abs(revolutions) * float64(m.cfg.TicksPerRotation))
	ticksPerSecond := int(math.Abs(rpm) * float64(m.cfg.TicksPerRotation) / 60.0)

	powerPct := 0.003
	if math.Signbit(rpm) != math.Signbit(revolutions) {
		ticks *= -1
		powerPct *= -1
		// ticksPerSecond *= 1
	}

	err := m.SetPower(ctx, powerPct)
	if err != nil {
		return err
	}
	_, err = m.b.runCommand(fmt.Sprintf("motor-gofor %s %d %d", m.name, ticks, ticksPerSecond))
	return err
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *arduinoMotor) GetPosition(ctx context.Context) (float64, error) {
	res, err := m.b.runCommand("motor-position " + m.name)
	if err != nil {
		return 0, err
	}

	ticks, err := strconv.Atoi(res)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse # ticks (%s) : %w", res, err)
	}

	return float64(ticks) / float64(m.cfg.TicksPerRotation), nil
}

// GetFeatures returns the status of optional features supported by the motor.
func (m *arduinoMotor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *arduinoMotor) Stop(ctx context.Context) error {
	_, err := m.b.runCommand("motor-off " + m.name)
	return err
}

// IsPowered returns whether or not the motor is currently on.
func (m *arduinoMotor) IsPowered(ctx context.Context) (bool, error) {
	res, err := m.b.runCommand("motor-ison " + m.name)
	if err != nil {
		return false, err
	}
	return res[0] == 't', nil
}

// GoTo instructs motor to go to a given position at a given RPM. Regardless of the directionality of the RPM this function will move the
// motor towards the specified target.
func (m *arduinoMotor) GoTo(ctx context.Context, rpm float64, target float64) error {
	ticks := int(target * float64(m.cfg.TicksPerRotation))
	ticksPerSecond := int(math.Abs(rpm) * float64(m.cfg.TicksPerRotation) / 60.0)

	_, err := m.b.runCommand(fmt.Sprintf("motor-goto %s %d %d", m.name, ticks, ticksPerSecond))
	return err
}

// GoTillStop moves a motor until stopped. The "stop" mechanism is up to the underlying motor implementation.
// This is currently not supported for ardunio controlled motors.
func (m *arduinoMotor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("not supported")
}

func (m *arduinoMotor) ResetZeroPosition(ctx context.Context, offset float64) error {
	offsetTicks := int64(offset * float64(m.cfg.TicksPerRotation))
	_, err := m.b.runCommand(fmt.Sprintf("motor-zero %s %d", m.name, offsetTicks))
	return err
}

func (m *arduinoMotor) Close(ctx context.Context) error {
	return m.Stop(ctx)
}

type encoder struct {
	b    *arduinoBoard
	cfg  motor.Config
	name string
}

// Position returns the current position in terms of ticks.
func (e *encoder) GetPosition(ctx context.Context) (int64, error) {
	res, err := e.b.runCommand("motor-position " + e.name)
	if err != nil {
		return 0, err
	}

	ticks, err := strconv.ParseInt(res, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse # ticks (%s) : %w", res, err)
	}

	return ticks, nil
}

// Start starts a background thread to run the encoder, if there is none needed this is a no-op.
func (e *encoder) Start(cancelCtx context.Context, activeBackgroundWorkers *sync.WaitGroup, onStart func()) {
	// no-op for arduino
	onStart()
}

func (e *encoder) ResetZeroPosition(ctx context.Context, offset int64) error {
	_, err := e.b.runCommand(fmt.Sprintf("motor-zero %s %d", e.name, offset))
	return err
}
