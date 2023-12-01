// Package roboclaw is the driver for the roboclaw motor drivers
// NOTE: This implementation is experimental and incomplete. Expect backward-breaking changes.
package roboclaw

/* Manufacturer:  		    Basicmicro
Supported Models: 	Roboclaw 2x7A, Roboclaw 2x15A (other models have not been tested)
Resources:
	 2x7A DataSheet: https://downloads.basicmicro.com/docs/roboclaw_datasheet_2x7A.pdf
	 2x15A DataSheet: https://downloads.basicmicro.com/docs/roboclaw_datasheet_2x15A.pdf
	 User Manual: https://downloads.basicmicro.com/docs/roboclaw_user_manual.pdf

This driver can connect to the roboclaw DC motor controller using a usb connection given as a serial path.
Note that the roboclaw must be initialized using the BasicMicro Motion Studio application prior to use.
The roboclaw must be in packet serial mode. The default address is 128.
Encoders can be attached to the roboclaw controller using the EN1 and EN2 pins. If encoders are connected,
update the ticks_per_rotation field in the config.

Configuration:
Motor Channel: specfies the channel the motor is connected to on the controller (1 or 2)
Serial baud rate: default of the roboclaw is 38400
Serial path: path to serial file
*/

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/CPRT/roboclaw"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

var (
	model               = resource.DefaultModelFamily.WithModel("roboclaw")
	connections         map[string]*roboclaw.Roboclaw
	baudRates           map[*roboclaw.Roboclaw]int
	validBaudRates      = []uint{460800, 230400, 115200, 57600, 38400, 19200, 9600, 2400}
	newConnectionNeeded bool
)

// Note that this maxRPM value was determined through very limited testing.
const (
	maxRPM      = 250
	minutesToMS = 60000
)

// Config is used for converting motor config attributes.
type Config struct {
	SerialPath       string `json:"serial_path"`
	SerialBaud       int    `json:"serial_baud_rate"`
	Channel          int    `json:"motor_channel"` // this is 1 or 2
	Address          int    `json:"address,omitempty"`
	TicksPerRotation int    `json:"ticks_per_rotation,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.Channel < 1 || conf.Channel > 2 {
		return nil, conf.wrongChannelError()
	}
	if conf.SerialPath == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	if conf.Address != 0 && (conf.Address < 128 || conf.Address > 135) {
		return nil, errors.New("serial address must be between 128 and 135")
	}

	if conf.TicksPerRotation < 0 {
		return nil, resource.NewConfigValidationError(path, errors.New("Ticks Per Rotation must be a positive number"))
	}

	if !rutils.ValidateBaudRate(validBaudRates, conf.SerialBaud) {
		return nil, resource.NewConfigValidationError(path, errors.Errorf("Baud rate invalid, must be one of these values: %v", validBaudRates))
	}
	return nil, nil
}

// Reconfigure automatically reconfigures the roboclaw when the config changes.
func (m *roboclawMotor) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConnectionNeeded = false
	newConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	if m.conf.TicksPerRotation != newConfig.TicksPerRotation {
		m.conf.TicksPerRotation = newConfig.TicksPerRotation
	}

	if m.conf.SerialBaud != newConfig.SerialBaud {
		m.conf.SerialBaud = newConfig.SerialBaud
		newConnectionNeeded = true
	}

	if m.conf.SerialPath != newConfig.SerialPath {
		m.conf.SerialBaud = newConfig.SerialBaud
		newConnectionNeeded = true
	}

	if m.conf.Channel != newConfig.Channel {
		m.conf.Channel = newConfig.Channel
	}

	if newConfig.Address != 0 && m.conf.Address != newConfig.Address {
		m.conf.Address = newConfig.Address
		m.addr = uint8(newConfig.Address)
		newConnectionNeeded = true
	}

	if newConnectionNeeded {
		conn, err := getOrCreateConnection(newConfig)
		if err != nil {
			return err
		}
		m.conn = conn
	}

	return nil
}

func (conf *Config) wrongChannelError() error {
	return fmt.Errorf("roboclaw motor channel has to be 1 or 2, but is %d", conf.Channel)
}

func init() {
	connections = make(map[string]*roboclaw.Roboclaw)
	baudRates = make(map[*roboclaw.Roboclaw]int)
	resource.RegisterComponent(
		motor.API,
		model,
		resource.Registration[motor.Motor, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (motor.Motor, error) {
				return newRoboClaw(conf, logger)
			},
		},
	)
}

func getOrCreateConnection(config *Config) (*roboclaw.Roboclaw, error) {
	// Check if there is already a roboclaw motor connection with the same serial config. This allows
	// multiple motors to share the same controller without stepping on each other.
	connection, ok := connections[config.SerialPath]
	if !ok {
		c := &roboclaw.Config{Name: config.SerialPath, Retries: 3}
		if config.SerialBaud > 0 {
			c.Baud = config.SerialBaud
		}
		newConn, err := roboclaw.Init(c)
		if err != nil {
			return nil, err
		}
		connections[config.SerialPath] = newConn
		baudRates[newConn] = config.SerialBaud
		return newConn, nil
	}

	if baudRates[connection] != config.SerialBaud {
		return nil, errors.New("cannot have multiple roboclaw motors with different baud rates")
	}
	return connection, nil
}

func newRoboClaw(conf resource.Config, logger logging.Logger) (motor.Motor, error) {
	motorConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	if motorConfig.Channel < 1 || motorConfig.Channel > 2 {
		return nil, motorConfig.wrongChannelError()
	}

	if motorConfig.Address == 0 {
		motorConfig.Address = 128
	}

	c, err := getOrCreateConnection(motorConfig)
	if err != nil {
		return nil, err
	}

	return &roboclawMotor{
		Named:  conf.ResourceName().AsNamed(),
		conn:   c,
		conf:   motorConfig,
		addr:   uint8(motorConfig.Address),
		logger: logger,
		opMgr:  operation.NewSingleOperationManager(),
		maxRPM: maxRPM,
	}, nil
}

type roboclawMotor struct {
	resource.Named
	resource.TriviallyCloseable
	conn *roboclaw.Roboclaw
	conf *Config

	addr   uint8
	maxRPM float64

	logger logging.Logger
	opMgr  *operation.SingleOperationManager

	powerPct float64
}

func (m *roboclawMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)

	if powerPct > 1 {
		powerPct = 1
	} else if powerPct < -1 {
		powerPct = -1
	}

	switch m.conf.Channel {
	case 1:
		m.powerPct = powerPct
		return m.conn.DutyM1(m.addr, int16(powerPct*32767))
	case 2:
		m.powerPct = powerPct
		return m.conn.DutyM2(m.addr, int16(powerPct*32767))
	default:
		return m.conf.wrongChannelError()
	}
}

func goForMath(rpm, revolutions float64) (float64, time.Duration) {
	// If revolutions is 0, the returned wait duration will be 0 representing that
	// the motor should run indefinitely.
	if revolutions == 0 {
		powerPct := 1.0
		return powerPct, 0
	}

	dir := rpm * revolutions / math.Abs(revolutions*rpm)
	powerPct := math.Abs(rpm) / maxRPM * dir
	waitDur := time.Duration(math.Abs(revolutions/rpm)*minutesToMS) * time.Millisecond
	return powerPct, waitDur
}

func (m *roboclawMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	speed := math.Abs(rpm)
	if speed < 0.1 {
		m.logger.CWarn(ctx, "motor speed is nearly 0 rev_per_min")
		return motor.NewZeroRPMError()
	}

	// If no encoders present, distance traveled is estimated based on max RPM.
	if m.conf.TicksPerRotation == 0 {
		if rpm > maxRPM {
			rpm = maxRPM
		} else if rpm < -1*maxRPM {
			rpm = -1 * maxRPM
		}
		powerPct, waitDur := goForMath(rpm, revolutions)
		m.logger.CInfo(ctx, "distance traveled is a time based estimation with max RPM 250. For increased accuracy, connect encoders")
		err := m.SetPower(ctx, powerPct, extra)
		if err != nil {
			return errors.Wrap(err, "error in GoFor")
		}
		if revolutions == 0 {
			return nil
		}
		if m.opMgr.NewTimedWaitOp(ctx, waitDur) {
			return m.Stop(ctx, extra)
		}
	}

	ctx, done := m.opMgr.New(ctx)
	defer done()

	ticks := uint32(revolutions * float64(m.conf.TicksPerRotation))
	ticksPerSecond := int32((rpm * float64(m.conf.TicksPerRotation)) / 60)

	var err error

	switch m.conf.Channel {
	case 1:
		err = m.conn.SpeedDistanceM1(m.addr, ticksPerSecond, ticks, true)
	case 2:
		err = m.conn.SpeedDistanceM2(m.addr, ticksPerSecond, ticks, true)
	default:
		return m.conf.wrongChannelError()
	}
	if err != nil {
		return err
	}
	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop)
}

func (m *roboclawMotor) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	if m.conf.TicksPerRotation == 0 {
		return errors.New("roboclaw needs an encoder connected to use GoTo")
	}
	pos, err := m.Position(ctx, extra)
	if err != nil {
		return err
	}
	return m.GoFor(ctx, rpm, positionRevolutions-pos, extra)
}

func (m *roboclawMotor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	newTicks := int32(-1 * offset * float64(m.conf.TicksPerRotation))
	switch m.conf.Channel {
	case 1:
		return m.conn.SetEncM1(m.addr, newTicks)
	case 2:
		return m.conn.SetEncM2(m.addr, newTicks)
	default:
		return m.conf.wrongChannelError()
	}
}

func (m *roboclawMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	var ticks uint32
	var err error

	switch m.conf.Channel {
	case 1:
		ticks, _, err = m.conn.ReadEncM1(m.addr)
	case 2:
		ticks, _, err = m.conn.ReadEncM2(m.addr)
	default:
		return 0, m.conf.wrongChannelError()
	}
	if err != nil {
		return 0, err
	}
	return float64(ticks) / float64(m.conf.TicksPerRotation), nil
}

func (m *roboclawMotor) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{
		PositionReporting: true,
	}, nil
}

func (m *roboclawMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	return m.SetPower(ctx, 0, extra)
}

func (m *roboclawMotor) IsMoving(ctx context.Context) (bool, error) {
	on, _, err := m.IsPowered(ctx, nil)
	return on, err
}

func (m *roboclawMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	pow1, pow2, err := m.conn.ReadPWMs(m.addr)
	if err != nil {
		return false, 0.0, err
	}
	switch m.conf.Channel {
	case 1:
		return pow1 != 0, m.powerPct, nil
	case 2:
		return pow2 != 0, m.powerPct, nil
	default:
		return false, 0.0, m.conf.wrongChannelError()
	}
}
