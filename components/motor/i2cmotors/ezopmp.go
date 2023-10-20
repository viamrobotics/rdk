//go:build linux

// Package ezopmp is a motor driver for the hydrogarden pump
package ezopmp

import (
	"bytes"
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

// Config is user config inputs for ezopmp.
type Config struct {
	BusName     string `json:"i2c_bus"`
	I2CAddress  *byte  `json:"i2c_addr"`
	MaxReadBits *int   `json:"max_read_bits"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string

	if conf.BusName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "bus_name")
	}

	if conf.I2CAddress == nil {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c_address")
	}

	if conf.MaxReadBits == nil {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "max_read_bits")
	}

	return deps, nil
}

var model = resource.DefaultModelFamily.WithModel("ezopmp")

func init() {
	resource.RegisterComponent(motor.API, model, resource.Registration[motor.Motor, *Config]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.ZapCompatibleLogger,
		) (motor.Motor, error) {
			newConf, err := resource.NativeConfig[*Config](conf)
			if err != nil {
				return nil, err
			}
			return NewMotor(ctx, deps, newConf, conf.ResourceName(), logging.FromZapCompatible(logger))
		},
	})
}

// Ezopmp represents a motor connected via the I2C protocol.
type Ezopmp struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	bus         board.I2C
	I2CAddress  byte
	maxReadBits int
	logger      logging.Logger
	maxPowerPct float64
	powerPct    float64
	maxFlowRate float64
	opMgr       *operation.SingleOperationManager
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
func NewMotor(ctx context.Context, deps resource.Dependencies, c *Config, name resource.Name,
	logger logging.Logger,
) (motor.Motor, error) {
	bus, err := genericlinux.NewI2cBus(c.BusName)
	if err != nil {
		return nil, err
	}

	m := &Ezopmp{
		Named:       name.AsNamed(),
		bus:         bus,
		I2CAddress:  *c.I2CAddress,
		maxReadBits: *c.MaxReadBits,
		logger:      logger,
		maxPowerPct: 1.0,
		powerPct:    0.0,
		opMgr:       operation.NewSingleOperationManager(),
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
	switch speed := math.Abs(mLPerMin); {
	case speed < 0.1:
		m.logger.Warn("motor speed is nearly 0 rev_per_min")
		return motor.NewZeroRPMError()
	case m.maxFlowRate > 0 && speed > m.maxFlowRate-0.1:
		m.logger.Warnf("motor speed is nearly the max rev_per_min (%f)", m.maxFlowRate)
	default:
	}

	ctx, done := m.opMgr.New(ctx)
	defer done()

	commandString := "DC," + strconv.FormatFloat(mLPerMin, 'f', -1, 64) + "," + strconv.FormatFloat(mins, 'f', -1, 64)
	command := []byte(commandString)
	if err := m.writeRegWithCheck(ctx, command); err != nil {
		return errors.Wrap(err, "error in GoFor")
	}

	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop)
}

// GoTo uses the Dose Over Time Command in the EZO-PMP datasheet
// mLPerMin = rpm, mins = revolutions.
func (m *Ezopmp) GoTo(ctx context.Context, mLPerMin, mins float64, extra map[string]interface{}) error {
	switch speed := math.Abs(mLPerMin); {
	case speed < 0.5:
		return errors.New("cannot move this slowly")
	case speed > 105:
		return errors.New("cannot move this fast")
	}

	commandString := "D," + strconv.FormatFloat(mLPerMin, 'f', -1, 64) + "," + strconv.FormatFloat(mins, 'f', -1, 64)
	command := []byte(commandString)
	if err := m.writeRegWithCheck(ctx, command); err != nil {
		return errors.Wrap(err, "error in GoTo")
	}
	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop)
}

// ResetZeroPosition clears the amount of volume that has been dispensed.
func (m *Ezopmp) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	m.logger.Warnf("cannot reset position of motor (%v) because position refers to the total volume dispensed", m.Name().ShortName())
	return motor.NewResetZeroPositionUnsupportedError(m.Name().ShortName())
}

// Position will return the total volume dispensed.
func (m *Ezopmp) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	command := []byte(totVolDispensed)
	writeErr := m.writeReg(ctx, command)
	if writeErr != nil {
		return 0, errors.Wrap(writeErr, "error in Position")
	}
	val, err := m.readReg(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "error in Position")
	}
	splitMsg := strings.Split(string(val), ",")
	floatVal, err := strconv.ParseFloat(splitMsg[1], 64)
	return floatVal, err
}

// Properties returns the status of optional properties on the motor.
func (m *Ezopmp) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{
		PositionReporting: true,
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
		return false, 0, errors.Wrap(writeErr, "error in IsPowered")
	}
	val, err := m.readReg(ctx)
	if err != nil {
		return false, 0, errors.Wrap(err, "error in IsPowered")
	}

	splitMsg := strings.Split(string(val), ",")

	pumpStatus, err := strconv.ParseFloat(splitMsg[2], 64)
	if err != nil {
		return false, 0, errors.Wrap(err, "error in IsPowered")
	}

	if pumpStatus == 1 || pumpStatus == -1 {
		return true, m.powerPct, nil
	}
	return false, 0.0, nil
}
