// Package roboclaw is the driver for the roboclaw motor drivers
package roboclaw

import (
	"context"
	"errors"
	"time"

	"github.com/CPRT/roboclaw"
	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
)

const modelname = "roboclaw"

type roboclawConfig struct {
	SerialPort       string `json:"serial_port"`
	Baud             int
	Number           int // this is 1 or 2
	Address          int `json:"address,omitempty"`
	TicksPerRotation int `json:"ticks_per_rotation"`
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
			var conf roboclawConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&roboclawConfig{},
	)
}

func getOrCreateConnection(deps registry.Dependencies, config *roboclawConfig) (*roboclaw.Roboclaw, error) {
	for _, res := range deps {
		m, ok := utils.UnwrapProxy(res).(*roboclawMotor)
		if !ok {
			continue
		}
		if m.conf.SerialPort != config.SerialPort {
			continue
		}
		if m.conf.Baud != config.Baud {
			return nil, errors.New("cannot have multiple roboclaw motors with different baud")
		}
		return m.conn, nil
	}

	c := &roboclaw.Config{Name: config.SerialPort, Retries: 3}
	if config.Baud > 0 {
		c.Baud = config.Baud
	}
	return roboclaw.Init(c)
}

func newRoboClaw(deps registry.Dependencies, config config.Component, logger golog.Logger) (motor.Motor, error) {
	motorConfig, ok := config.ConvertedAttributes.(*roboclawConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(motorConfig, config.ConvertedAttributes)
	}

	if motorConfig.Number < 1 || motorConfig.Number > 2 {
		return nil, errors.New("roboclawConfig Number has to be 1 or 2")
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
	conf *roboclawConfig

	addr uint8

	logger golog.Logger
	opMgr  operation.SingleOperationManager

	generic.Unimplemented
}

func (m *roboclawMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)

	switch m.conf.Number {
	case 1:
		return m.conn.DutyM1(m.addr, int16(powerPct*32767))
	case 2:
		return m.conn.DutyM2(m.addr, int16(powerPct*32767))
	default:
		panic("impossible")
	}
}

func (m *roboclawMotor) GoFor(ctx context.Context, rpm float64, revolutions float64, extra map[string]interface{}) error {
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
		panic("impossible")
	}
	if err != nil {
		return err
	}
	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m)
}

func (m *roboclawMotor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	pos, err := m.GetPosition(ctx, extra)
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
		panic("impossible")
	}
}

func (m *roboclawMotor) GetPosition(ctx context.Context, extra map[string]interface{}) (float64, error) {
	var ticks uint32
	var err error

	switch m.conf.Number {
	case 1:
		ticks, _, err = m.conn.ReadEncM1(m.addr)
	case 2:
		ticks, _, err = m.conn.ReadEncM2(m.addr)
	default:
		panic("impossible")
	}
	if err != nil {
		return 0, err
	}
	return float64(ticks) / float64(m.conf.TicksPerRotation), nil
}

func (m *roboclawMotor) GetFeatures(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

func (m *roboclawMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	return m.SetPower(ctx, 0, extra)
}

func (m *roboclawMotor) IsMoving(ctx context.Context) (bool, error) {
	return m.IsPowered(ctx, nil)
}

func (m *roboclawMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, error) {
	pow1, pow2, err := m.conn.ReadPWMs(m.addr)
	if err != nil {
		return false, err
	}
	switch m.conf.Number {
	case 1:
		return pow1 == 0, nil
	case 2:
		return pow2 == 0, nil
	default:
		panic("impossible")
	}
}

func (m *roboclawMotor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return motor.NewGoTillStopUnsupportedError("(name unavailable)")
}
