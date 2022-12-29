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
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

// AttrConfig is user config inputs for ezopmp.
type AttrConfig struct {
	BoardName   string `json:"board"`
	BusName     string `json:"i2c_bus"`
	I2CAddress  *byte  `json:"i2c_addr"`
	MaxReadBits *int   `json:"max_read_bits"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	if config.BoardName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	if config.BusName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "bus_name")
	}

	if config.I2CAddress == nil {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c_address")
	}

	if config.MaxReadBits == nil {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "max_read_bits")
	}

	deps = append(deps, config.BoardName)
	return deps, nil
}

var modelName = resource.NewDefaultModel("ezopmp")

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewMotor(ctx, deps, config.ConvertedAttributes.(*AttrConfig), config.Name, logger)
		},
	}
	registry.RegisterComponent(motor.Subtype, modelName, _motor)
	config.RegisterComponentAttributeMapConverter(
		motor.Subtype,
		modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Squash: true, Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		}, &AttrConfig{})
}

// Ezopmp represents a motor connected via the I2C protocol.
type Ezopmp struct {
	motorName   string
	board       board.Board
	bus         board.I2C
	I2CAddress  byte
	maxReadBits int
	logger      golog.Logger
	maxPowerPct float64
	powerPct    float64
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
func NewMotor(ctx context.Context, deps registry.Dependencies, c *AttrConfig, name string,
	logger golog.Logger,
) (motor.LocalMotor, error) {
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
		I2CAddress:  *c.I2CAddress,
		maxReadBits: *c.MaxReadBits,
		logger:      logger,
		maxPowerPct: 1.0,
		powerPct:    0.0,
		motorName:   name,
	}

	flowRate, err := m.findMaxFlowRate(ctx)
	if err != nil {
		return nil, errors.Errorf("can't find max flow rate: %v", err)
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
		m.logger.Warn("i2c address set at 103")
		m.I2CAddress = 103
	}

	if m.maxReadBits == 0 {
		m.logger.Warn("max_read_bits set to 39")
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
func (m *Ezopmp) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)

	powerPct = math.Min(powerPct, m.maxPowerPct)
	powerPct = math.Max(powerPct, -1*m.maxPowerPct)
	m.powerPct = powerPct

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
func (m *Ezopmp) GoFor(ctx context.Context, mLPerMin, mins float64, extra map[string]interface{}) error {
	if mLPerMin == 0 {
		return motor.NewZeroRPMError() // Not strictly RPMs, but same idea
	}
	ctx, done := m.opMgr.New(ctx)
	defer done()

	switch speed := math.Abs(mLPerMin); {
	case speed < 0.5:
		return errors.Errorf("motor (%s) cannot move this slowly", m.motorName)
	case speed > m.maxFlowRate:
		return errors.Errorf("max continuous flow rate of motor (%s) is: %f", m.motorName, m.maxFlowRate)
	}

	commandString := "DC," + strconv.FormatFloat(mLPerMin, 'f', -1, 64) + "," + strconv.FormatFloat(mins, 'f', -1, 64)
	command := []byte(commandString)
	if err := m.writeRegWithCheck(ctx, command); err != nil {
		return errors.Wrapf(err, "error in GoFor from motor (%s)", m.motorName)
	}

	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop)
}

// GoTo uses the Dose Over Time Command in the EZO-PMP datasheet
// mLPerMin = rpm, mins = revolutions.
func (m *Ezopmp) GoTo(ctx context.Context, mLPerMin, mins float64, extra map[string]interface{}) error {
	switch speed := math.Abs(mLPerMin); {
	case speed < 0.5:
		return errors.Errorf("motor (%s) cannot move this slowly", m.motorName)
	case speed > 105:
		return errors.Errorf("motor (%s) cannot move this fast", m.motorName)
	}

	commandString := "D," + strconv.FormatFloat(mLPerMin, 'f', -1, 64) + "," + strconv.FormatFloat(mins, 'f', -1, 64)
	command := []byte(commandString)
	if err := m.writeRegWithCheck(ctx, command); err != nil {
		return errors.Wrapf(err, "error in GoTo from motor (%s)", m.motorName)
	}
	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop)
}

// ResetZeroPosition clears the amount of volume that has been dispensed.
func (m *Ezopmp) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	command := []byte(clear)
	return m.writeRegWithCheck(ctx, command)
}

// Position will return the total volume dispensed.
func (m *Ezopmp) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	command := []byte(totVolDispensed)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return 0, errors.Wrapf(writeErr, "error in Position from motor (%s)", m.motorName)
	}
	val, err := m.readReg(ctx)
	if err != nil {
		return 0, errors.Wrapf(err, "error in Position from motor (%s)", m.motorName)
	}
	splitMsg := strings.Split(string(val), ",")
	floatVal, err := strconv.ParseFloat(splitMsg[1], 64)
	return floatVal, err
}

// Properties returns the status of optional features on the motor.
func (m *Ezopmp) Properties(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *Ezopmp) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)
	command := []byte(stop)
	return m.writeRegWithCheck(ctx, command)
}

// IsMoving returns whether or not the motor is currently moving.
func (m *Ezopmp) IsMoving(ctx context.Context) (bool, error) {
	on, _, err := m.IsPowered(ctx, nil)
	return on, err
}

// IsPowered returns whether or not the motor is currently on, and how much power it's getting.
func (m *Ezopmp) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	command := []byte(dispenseStatus)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return false, 0, errors.Wrapf(writeErr, "error in IsPowered from motor (%s)", m.motorName)
	}
	val, err := m.readReg(ctx)
	if err != nil {
		return false, 0, errors.Wrapf(err, "error in IsPowered from motor (%s)", m.motorName)
	}

	splitMsg := strings.Split(string(val), ",")

	pumpStatus, err := strconv.ParseFloat(splitMsg[2], 64)
	if err != nil {
		return false, 0, errors.Wrapf(err, "error in IsPowered from motor (%s)", m.motorName)
	}

	if pumpStatus == 1 || pumpStatus == -1 {
		return true, m.powerPct, nil
	}
	return false, 0.0, nil
}

// GoTillStop is unimplemented.
func (m *Ezopmp) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return motor.NewGoTillStopUnsupportedError(m.motorName)
}
