// Package roboclaw is the driver for the roboclaw motor drivers
package roboclaw

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/CPRT/roboclaw"
	"github.com/edaniels/golog"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

const modelname = "roboclaw"

// AttrConfig is used for converting motor config attributes.
type AttrConfig struct {
	SerialPath       string `json:"serial_path"`
	SerialBaud       int    `json:"serial_baud_rate"`
	Number           int    `json:"number_of_motors"` // this is 1 or 2
	Address          int    `json:"address,omitempty"`
	TicksPerRotation int    `json:"ticks_per_rotation,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	if config.Number < 1 || config.Number > 2 {
		return config.wrongNumberError()
	}
	if config.SerialPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}

	if config.SerialBaud <= 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_baud_rate")
	}

	return nil
}

func (config *AttrConfig) wrongNumberError() error {
	return fmt.Errorf("roboclawConfig Number has to be 1 or 2, but is %d", config.Number)
}

func init() {
	registry.RegisterComponent(
		motor.Subtype,
		modelname,
		registry.Component{
			Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
				return newRoboClaw(deps, config, logger)
			},
		},
	)

	config.RegisterComponentAttributeMapConverter(
		motor.SubtypeName,
		modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{},
	)
}

func getOrCreateConnection(deps registry.Dependencies, config *AttrConfig) (*roboclaw.Roboclaw, error) {
	for _, res := range deps {
		m, ok := rdkutils.UnwrapProxy(res).(*roboclawMotor)
		if !ok {
			continue
		}
		if m.conf.SerialPath != config.SerialPath {
			continue
		}
		if m.conf.SerialBaud != config.SerialBaud {
			return nil, errors.New("cannot have multiple roboclaw motors with different baud")
		}
		return m.conn, nil
	}

	c := &roboclaw.Config{Name: config.SerialPath, Retries: 3}
	if config.SerialBaud > 0 {
		c.Baud = config.SerialBaud
	}
	return roboclaw.Init(c)
}

func newRoboClaw(deps registry.Dependencies, config config.Component, logger golog.Logger) (motor.Motor, error) {
	motorConfig, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(motorConfig, config.ConvertedAttributes)
	}

	if motorConfig.Number < 1 || motorConfig.Number > 2 {
		return nil, motorConfig.wrongNumberError()
	}

	if motorConfig.Address == 0 {
		motorConfig.Address = 128
	}

	if motorConfig.TicksPerRotation <= 0 {
		motorConfig.TicksPerRotation = 1
	}

	c, err := getOrCreateConnection(deps, motorConfig)
	if err != nil {
		return nil, err
	}

	return &roboclawMotor{conn: c, conf: motorConfig, addr: uint8(motorConfig.Address), logger: logger}, nil
}

var _ = motor.LocalMotor(&roboclawMotor{})

type roboclawMotor struct {
	conn *roboclaw.Roboclaw
	conf *AttrConfig

	addr uint8

	logger golog.Logger
	opMgr  operation.SingleOperationManager

	powerPct float64

	generic.Unimplemented
}

func (m *roboclawMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)

	powerPct = math.Min(powerPct, 1.0)
	powerPct = math.Max(powerPct, 0.0)

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

func (m *roboclawMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	if rpm == 0 {
		return motor.NewZeroRPMError()
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
	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m)
}

func (m *roboclawMotor) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
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
		return pow1 == 0, m.powerPct, nil
	case 2:
		return pow2 == 0, m.powerPct, nil
	default:
		return false, 0.0, m.conf.wrongNumberError()
	}
}

func (m *roboclawMotor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return motor.NewGoTillStopUnsupportedError("(name unavailable)")
}
