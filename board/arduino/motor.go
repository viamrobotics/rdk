package arduino

import (
	"context"
	"fmt"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"
)

// init registers an arduino motor.
func init() {
	registry.RegisterMotor(modelName, registry.Motor{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error) {
		motorConfig := config.ConvertedAttributes.(*motor.Config)
		if motorConfig.BoardName == "" {
			return nil, errors.New("expected board name in config for motor")
		}
		b, ok := r.BoardByName(motorConfig.BoardName)
		if !ok {
			return nil, fmt.Errorf("expected to find board %q", motorConfig.BoardName)
		}
		// Note(erd): this would not be needed if encoders were a component
		actualBoard, ok := utils.UnwrapProxy(b).(*arduinoBoard)
		if !ok {
			return nil, errors.New("expected board to be an arduino board")
		}

		return actualBoard.configureMotor(config, motorConfig)
	}})
	motor.RegisterConfigAttributeConverter(modelName)
}

func (b *arduinoBoard) configureMotor(config config.Component, motorConfig *motor.Config) (motor.Motor, error) {
	if !((motorConfig.Pins["pwm"] != "" && motorConfig.Pins["dir"] != "") || (motorConfig.Pins["a"] != "" || motorConfig.Pins["b"] != "")) {
		return nil, errors.New("arduino needs at least a & b, or dir & pwm pins")
	}

	if motorConfig.Encoder == "" || motorConfig.EncoderB == "" {
		return nil, errors.New("arduino needs a and b hall encoders")
	}

	if motorConfig.TicksPerRotation <= 0 {
		return nil, errors.New("arduino motors TicksPerRotation to be set")
	}

	for _, pin := range []string{"pwm", "a", "b", "dir", "en"} {
		if _, ok := motorConfig.Pins[pin]; !ok {
			motorConfig.Pins[pin] = "-1"
		}
	}
	cmd := fmt.Sprintf("config-motor-dc %s %s %s %s %s %s e %s %s",
		config.Name,
		motorConfig.Pins["pwm"], // Optional if using A/B inputs (one of them will be PWMed if missing)
		motorConfig.Pins["a"],   // Use either A & B, or DIR inputs, never both
		motorConfig.Pins["b"],   // (A & B [& PWM] ) || (DIR & PWM)
		motorConfig.Pins["dir"], // PWM is also required when using DIR
		motorConfig.Pins["en"],  // Always optional, inverting input (LOW = ENABLED)
		motorConfig.Encoder,
		motorConfig.EncoderB,
	)

	res, err := b.runCommand(cmd)
	if err != nil {
		return nil, err
	}

	if res != "ok" {
		return nil, fmt.Errorf("got unknown response when configureMotor %s", res)
	}

	m, err := board.NewEncodedMotor(config, *motorConfig, &arduinoMotor{b, *motorConfig, config.Name}, &encoder{b, *motorConfig, config.Name}, b.logger)
	if err != nil {
		return nil, err
	}
	if motorConfig.Pins["pwm"] != "-1" && motorConfig.PWMFreq > 0 {
		//When the motor controller has a PWM pin exposed (either (A && B && PWM) || (DIR && PWM))
		//We control the motor speed with the PWM pin
		err = b.pwmSetFreqArduino(motorConfig.Pins["pwm"], motorConfig.PWMFreq)
		if err != nil {
			return nil, err
		}
	} else if (motorConfig.Pins["a"] != "-1" && motorConfig.Pins["b"] != "-1") && motorConfig.PWMFreq > 0 {
		// When the motor controller only exposes A & B pin
		// We control the motor speed with both pins
		err = b.pwmSetFreqArduino(motorConfig.Pins["a"], motorConfig.PWMFreq)
		if err != nil {
			return nil, err
		}
		err = b.pwmSetFreqArduino(motorConfig.Pins["b"], motorConfig.PWMFreq)
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

// Power sets the percentage of power the motor should employ between 0-1.
func (m *arduinoMotor) Power(ctx context.Context, powerPct float32) error {
	if powerPct <= .001 {
		return m.Off(ctx)
	}

	_, err := m.b.runCommand(fmt.Sprintf("motor-power %s %d", m.name, int(255.0*powerPct)))
	return err
}

// Go instructs the motor to go in a specific direction at a percentage
// of power between 0-1.
func (m *arduinoMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	if powerPct <= 0 {
		return m.Off(ctx)
	}

	var dir string
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		dir = "f"
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		dir = "n"
	default:
		return m.Off(ctx)
	}

	_, err := m.b.runCommand(fmt.Sprintf("motor-go %s %s %d", m.name, dir, int(255.0*powerPct)))
	return err
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute.
func (m *arduinoMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	ticks := int(revolutions * float64(m.cfg.TicksPerRotation))
	ticksPerSecond := int(rpm * float64(m.cfg.TicksPerRotation) / 60.0)
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
		// no-op
	} else if d == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		ticks *= -1
	} else {
		return errors.New("unknown direction")
	}

	_, err := m.b.runCommand(fmt.Sprintf("motor-gofor %s %d %d", m.name, ticks, ticksPerSecond))
	return err
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *arduinoMotor) Position(ctx context.Context) (float64, error) {
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

// PositionSupported returns whether or not the motor supports reporting of its position which
// is reliant on having an encoder.
func (m *arduinoMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

// Off turns the motor off.
func (m *arduinoMotor) Off(ctx context.Context) error {
	_, err := m.b.runCommand("motor-off " + m.name)
	return err
}

// IsOn returns whether or not the motor is currently on.
func (m *arduinoMotor) IsOn(ctx context.Context) (bool, error) {
	res, err := m.b.runCommand("motor-ison " + m.name)
	if err != nil {
		return false, err
	}
	return res[0] == 't', nil
}

func (m *arduinoMotor) GoTo(ctx context.Context, rpm float64, target float64) error {
	ticks := int(target * float64(m.cfg.TicksPerRotation))
	ticksPerSecond := int(rpm * float64(m.cfg.TicksPerRotation) / 60.0)
	_, err := m.b.runCommand(fmt.Sprintf("motor-goto %s %d %d", m.name, ticks, ticksPerSecond))
	return err
}

func (m *arduinoMotor) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("not supported")
}

func (m *arduinoMotor) Zero(ctx context.Context, offset float64) error {
	offsetTicks := int64(offset * float64(m.cfg.TicksPerRotation))
	_, err := m.b.runCommand(fmt.Sprintf("motor-zero %s %d", m.name, offsetTicks))
	return err
}

func (m *arduinoMotor) Close() error {
	return m.Off(context.Background())
}
