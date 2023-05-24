// Package roboclaw is the driver for the roboclaw motor drivers
// NOTE: This implementation is experimental and incomplete. Expect backward-breaking changes.
package roboclaw

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/CPRT/roboclaw"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

var (
	model       = resource.DefaultModelFamily.WithModel("roboclaw")
	connections map[string]*roboclaw.Roboclaw
	baudRates   map[*roboclaw.Roboclaw]int
)

const maxRPM = 200

// Config is used for converting motor config attributes.
type Config struct {
	SerialPath       string `json:"serial_path"`
	SerialBaud       int    `json:"serial_baud_rate"`
	Number           int    `json:"number_of_motors"` // this is 1 or 2
	Address          int    `json:"address,omitempty"`
	TicksPerRotation int    `json:"ticks_per_rotation,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.Number < 1 || conf.Number > 2 {
		return nil, conf.wrongNumberError()
	}
	if conf.SerialPath == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}

	if conf.SerialBaud <= 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "serial_baud_rate")
	}
	if conf.Address != 0 && (conf.Address < 128 || conf.Address > 135) {
		return nil, errors.New("serial address must be between 128 and 135")
	}

	if conf.SerialBaud != 2400 && conf.SerialBaud != 9600 && conf.SerialBaud != 19200 && conf.SerialBaud != 38400 && conf.SerialBaud != 115200 {
		return nil, errors.New("invalid baud_rate, acceptable values are 2400, 9600, 19200, 38400, 115200")
	}
	return nil, nil
}

func (conf *Config) wrongNumberError() error {
	return fmt.Errorf("roboclawConfig Number has to be 1 or 2, but is %d", conf.Number)
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
				logger golog.Logger,
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
		return nil, errors.New("cannot have multiple roboclaw motors with different baud")
	}
	return connection, nil
}

func newRoboClaw(conf resource.Config, logger golog.Logger) (motor.Motor, error) {
	motorConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	if motorConfig.Number < 1 || motorConfig.Number > 2 {
		return nil, motorConfig.wrongNumberError()
	}

	if motorConfig.Address == 0 {
		motorConfig.Address = 128
	}

	if motorConfig.TicksPerRotation < 0 {
		motorConfig.TicksPerRotation = 1
	}

	c, err := getOrCreateConnection(motorConfig)
	if err != nil {
		return nil, err
	}

	motor := &roboclawMotor{
		Named:  conf.ResourceName().AsNamed(),
		conn:   c,
		conf:   motorConfig,
		addr:   uint8(motorConfig.Address),
		logger: logger,
		maxRPM: maxRPM,
	}

	return motor, nil
}

type roboclawMotor struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	conn *roboclaw.Roboclaw
	conf *Config

	addr   uint8
	maxRPM float64

	logger golog.Logger
	opMgr  operation.SingleOperationManager

	powerPct float64
}

func (m *roboclawMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)

	if powerPct > 1 {
		powerPct = 1
	} else if powerPct < -1 {
		powerPct = -1
	}

	switch m.conf.Number {
	case 1:
		m.powerPct = powerPct
		return m.conn.DutyM1(m.addr, int16(powerPct*32767))
	case 2:
		m.powerPct = powerPct
		return m.conn.DutyM2(m.addr, int16(powerPct*32767))
	default:
		return m.conf.wrongNumberError()
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
	waitDur := time.Duration(math.Abs(revolutions/rpm)*60*1000) * time.Millisecond
	return powerPct, waitDur
}

func (m *roboclawMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	speed := math.Abs(rpm)
	if speed < 0.1 {
		m.logger.Warn("motor speed is nearly 0 rev_per_min")
		return motor.NewZeroRPMError()
	}

	if rpm > maxRPM {
		rpm = maxRPM
	} else if rpm < -1*maxRPM {
		rpm = -1 * maxRPM
	}

	// If no encoders present, distance traveled is estimated based on max RPM.
	if m.conf.TicksPerRotation == 0 {
		powerPct, waitDur := goForMath(rpm, revolutions)
		m.logger.Info("distance traveled is a time based estimation with max RPM 200. For increased accuracy, connect encoders")
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

	switch m.conf.Number {
	case 1:
		err = m.conn.SpeedDistanceM1(m.addr, ticksPerSecond, ticks, true)
	case 2:
		err = m.conn.SpeedDistanceM2(m.addr, ticksPerSecond, ticks, true)
	default:
		return m.conf.wrongNumberError()
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
	newTicks := int32(offset * float64(m.conf.TicksPerRotation))
	switch m.conf.Number {
	case 1:
		return m.conn.SetEncM1(m.addr, newTicks)
	case 2:
		return m.conn.SetEncM2(m.addr, newTicks)
	default:
		return m.conf.wrongNumberError()
	}
}

func (m *roboclawMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	var ticks uint32
	var err error

	switch m.conf.Number {
	case 1:
		ticks, _, err = m.conn.ReadEncM1(m.addr)
	case 2:
		ticks, _, err = m.conn.ReadEncM2(m.addr)
	default:
		return 0, m.conf.wrongNumberError()
	}
	if err != nil {
		return 0, err
	}
	return float64(ticks) / float64(m.conf.TicksPerRotation), nil
}

func (m *roboclawMotor) Properties(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
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
	switch m.conf.Number {
	case 1:
		return pow1 != 0, m.powerPct, nil
	case 2:
		return pow2 != 0, m.powerPct, nil
	default:
		return false, 0.0, m.conf.wrongNumberError()
	}
}

func (m *roboclawMotor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return motor.NewGoTillStopUnsupportedError(m.Name().ShortName())
}
