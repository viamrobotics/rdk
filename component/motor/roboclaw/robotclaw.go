// Package roboclaw is the driver for the roboclaw motor drivers
package roboclaw

import (
	"context"
	"errors"

	"github.com/CPRT/roboclaw"
	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
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
			Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
				return newRoboClaw(r, config, logger)
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

func getOrCreateConnection(r robot.Robot, config *roboclawConfig) (*roboclaw.Roboclaw, error) {
	for _, n := range r.ResourceNames() {
		r, err := r.ResourceByName(n)
		if err != nil {
			continue
		}
		m, ok := utils.UnwrapProxy(r).(*roboclawMotor)
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

func newRoboClaw(r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error) {
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

	c, err := getOrCreateConnection(r, motorConfig)
	if err != nil {
		return nil, err
	}

	return &roboclawMotor{conn: c, conf: motorConfig, addr: uint8(motorConfig.Address), logger: logger}, nil
}

type roboclawMotor struct {
	conn *roboclaw.Roboclaw
	conf *roboclawConfig

	addr uint8

	logger golog.Logger

	generic.Unimplemented
}

func (m *roboclawMotor) SetPower(ctx context.Context, powerPct float64) error {
	switch m.conf.Number {
	case 1:
		return m.conn.DutyM1(m.addr, int16(powerPct*32767))
	case 2:
		return m.conn.DutyM2(m.addr, int16(powerPct*32767))
	default:
		panic("impossible")
	}
}

func (m *roboclawMotor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	ticks := uint32(revolutions * float64(m.conf.TicksPerRotation))
	ticksPerSecond := int32((rpm * float64(m.conf.TicksPerRotation)) / 60)

	switch m.conf.Number {
	case 1:
		return m.conn.SpeedDistanceM1(m.addr, ticksPerSecond, ticks, true)
	case 2:
		return m.conn.SpeedDistanceM1(m.addr, ticksPerSecond, ticks, true)
	default:
		panic("impossible")
	}
}

func (m *roboclawMotor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	pos, err := m.GetPosition(ctx)
	if err != nil {
		return err
	}
	return m.GoFor(ctx, rpm, positionRevolutions-pos)
}

func (m *roboclawMotor) ResetZeroPosition(ctx context.Context, offset float64) error {
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

func (m *roboclawMotor) GetPosition(ctx context.Context) (float64, error) {
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

func (m *roboclawMotor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

func (m *roboclawMotor) Stop(ctx context.Context) error {
	return m.SetPower(ctx, 0)
}

func (m *roboclawMotor) IsPowered(ctx context.Context) (bool, error) {
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
