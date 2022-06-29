// Package ezopmp is a motor driver for the hydrogarden pump
package ezopmp

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
)

// Config is user config inputs for ezopmp.
type Config struct {
	motor.Config
	BusName     string `json:"bus_name"`
	I2CAddress  byte   `json:"i2c_address"`
	MaxReadBits int    `json:"max_read_bits"`
}

const modelName = "ezopmp"

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewMotor(ctx, deps, config.ConvertedAttributes.(*Config), logger)
		},
	}
	registry.RegisterComponent(motor.Subtype, modelName, _motor)
	config.RegisterComponentAttributeMapConverter(
		motor.SubtypeName,
		modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Squash: true, Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		}, &Config{})
}

// Ezopmp represents a motor connected via the I2C protocol.
type Ezopmp struct {
	board       board.Board
	bus         board.I2C
	I2CAddress  byte
	maxReadBits int
	logger      golog.Logger
	maxPowerPct float64
	maxFlowRate float64
	opMgr       operation.SingleOperationManager
	generic.Unimplemented
}

// available commands.
const (
	dispenseStatus  = "D,?"
	stop            = "X"
	totVolDispensed = "TV,?"
	clear           = "clear"
	maxFlowRate     = "DC,?"
)

// NewMotor returns a motor(Ezopmp) with I2C protocol.
func NewMotor(ctx context.Context, deps registry.Dependencies, c *Config, logger golog.Logger) (motor.LocalMotor, error) {
	b, err := board.FromDependencies(deps, c.BoardName)
	if err != nil {
		return nil, err
	}

	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", c.BoardName)
	}
	bus, ok := localB.I2CByName(c.BusName)
	if !ok {
		return nil, errors.Errorf("can't find I2C bus (%s) requested by Motor", c.BusName)
	}

	m := &Ezopmp{
		board:       b,
		bus:         bus,
		I2CAddress:  c.I2CAddress,
		maxReadBits: c.MaxReadBits,
		logger:      logger,
		maxPowerPct: 1.0,
	}

	flowRate, err := m.findMaxFlowRate(ctx)
	if err != nil {
		return nil, errors.New("can't find max flow rate")
	}
	m.maxFlowRate = flowRate

	if err := m.Validate(); err != nil {
		return nil, err
	}

	return m, nil
}

// for this pump, it will return the total volume dispensed.
func (m *Ezopmp) findMaxFlowRate(ctx context.Context) (float64, error) {
	command := []byte(maxFlowRate)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return 0, writeErr
	}
	val, err := m.readReg(ctx)
	if err != nil {
		return 0, err
	}
	splitMsg := strings.Split(string(val), ",")
	flowRate, err := strconv.ParseFloat(splitMsg[1], 64)
	return flowRate, err
}

// Validate if this config is valid.
func (m *Ezopmp) Validate() error {
	if m.board == nil {
		return errors.New("need a board for ezopmp")
	}

	if m.bus == nil {
		return errors.New("need a bus for ezopmp")
	}

	if m.I2CAddress == 0 {
		m.I2CAddress = 103
	}

	if m.maxReadBits == 0 {
		m.maxReadBits = 39
	}

	if m.maxPowerPct > 1 {
		m.maxPowerPct = 1
	}

	if m.maxFlowRate == 0 {
		m.maxFlowRate = 50.5
	}
	return nil
}

func (m *Ezopmp) writeReg(ctx context.Context, command []byte) error {
	handle, err := m.bus.OpenHandle(m.I2CAddress)
	if err != nil {
		return err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			m.logger.Error(err)
		}
	}()

	return handle.Write(ctx, command)
}

func (m *Ezopmp) readReg(ctx context.Context) ([]byte, error) {
	handle, err := m.bus.OpenHandle(m.I2CAddress)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			m.logger.Error(err)
		}
	}()

	readVal := []byte{254, 0}
	for readVal[0] == 254 {
		readVal, err = handle.Read(ctx, m.maxReadBits)
		if err != nil {
			return nil, err
		}
	}

	switch readVal[0] {
	case 1:
		noF := bytes.Trim(readVal[1:], "\xff")
		return bytes.Trim(noF, "\x00"), nil
	case 2:
		return nil, errors.New("syntax error, code: 2")
	case 255:
		return nil, errors.New("no data to send, code: 255")
	case 254:
		return nil, errors.New("data not ready, code: 254")
	default:
		return nil, errors.Errorf("error code not understood %b", readVal[0])
	}
}

// helper function to write the command and then read to check if success.
func (m *Ezopmp) writeRegWithCheck(ctx context.Context, command []byte) error {
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return writeErr
	}
	_, readErr := m.readReg(ctx)
	return readErr
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
// Negative power implies a backward directional rotational
// for this pump, it goes between 0.5ml to 105ml/min.
func (m *Ezopmp) SetPower(ctx context.Context, powerPct float64) error {
	m.opMgr.CancelRunning(ctx)

	powerPct = math.Min(powerPct, m.maxPowerPct)
	powerPct = math.Max(powerPct, -1*m.maxPowerPct)

	var command []byte
	if powerPct == 0 {
		command = []byte(stop)
	} else {
		var powerVal float64
		if powerPct < 0 {
			powerVal = (powerPct * 104.5) - 0.5
		} else {
			powerVal = (powerPct * 104.5) + 0.5
		}
		stringVal := "DC," + strconv.FormatFloat(powerVal, 'f', -1, 64) + ",*"
		command = []byte(stringVal)
	}

	return m.writeRegWithCheck(ctx, command)
}

// GoFor sets a constant flow rate
// mLPerMin = rpm, mins = revolutions.
func (m *Ezopmp) GoFor(ctx context.Context, mLPerMin float64, mins float64) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	switch speed := math.Abs(mLPerMin); {
	case speed < 0.5:
		return errors.New("motor cannot move this slowly")
	case speed > m.maxFlowRate:
		return errors.Errorf("max continuous flow rate is: %f", m.maxFlowRate)
	}

	commandString := "DC," + strconv.FormatFloat(mLPerMin, 'f', -1, 64) + "," + strconv.FormatFloat(mins, 'f', -1, 64)
	command := []byte(commandString)
	if err := m.writeRegWithCheck(ctx, command); err != nil {
		return err
	}
	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m)
}

// GoTo uses the Dose Over Time Command in the EZO-PMP datasheet
// mLPerMin = rpm, mins = revolutions.
func (m *Ezopmp) GoTo(ctx context.Context, mLPerMin float64, mins float64) error {
	switch speed := math.Abs(mLPerMin); {
	case speed < 0.5:
		return errors.New("motor cannot move this slowly")
	case speed > 105:
		return errors.New("motor cannot move this fast")
	}

	commandString := "D," + strconv.FormatFloat(mLPerMin, 'f', -1, 64) + "," + strconv.FormatFloat(mins, 'f', -1, 64)
	command := []byte(commandString)
	if err := m.writeRegWithCheck(ctx, command); err != nil {
		return err
	}
	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m)
}

// ResetZeroPosition clears the amount of volume that has been dispensed.
func (m *Ezopmp) ResetZeroPosition(ctx context.Context, offset float64) error {
	command := []byte(clear)
	return m.writeRegWithCheck(ctx, command)
}

// GetPosition will return the total volume dispensed.
func (m *Ezopmp) GetPosition(ctx context.Context) (float64, error) {
	command := []byte(totVolDispensed)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return 0, writeErr
	}
	val, err := m.readReg(ctx)
	if err != nil {
		return 0, err
	}
	splitMsg := strings.Split(string(val), ",")
	floatVal, err := strconv.ParseFloat(splitMsg[1], 64)
	return floatVal, err
}

// GetFeatures returns the status of optional features on the motor.
func (m *Ezopmp) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *Ezopmp) Stop(ctx context.Context) error {
	m.opMgr.CancelRunning(ctx)
	command := []byte(stop)
	return m.writeRegWithCheck(ctx, command)
}

// IsMoving returns whether or not the motor is currently moving.
func (m *Ezopmp) IsMoving(ctx context.Context) (bool, error) {
	return m.IsPowered(ctx)
}

// IsPowered returns whether or not the motor is currently on.
func (m *Ezopmp) IsPowered(ctx context.Context) (bool, error) {
	command := []byte(dispenseStatus)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return false, writeErr
	}
	val, err := m.readReg(ctx)
	if err != nil {
		return false, err
	}

	splitMsg := strings.Split(string(val), ",")

	pumpStatus, err := strconv.ParseFloat(splitMsg[2], 64)
	if err != nil {
		return false, err
	}

	if pumpStatus == 1 || pumpStatus == -1 {
		return true, nil
	}
	return false, nil
}

// GoTillStop is unimplemented.
func (m *Ezopmp) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return motor.NewGoTillStopUnsupportedError("(name unavailable)")
}
