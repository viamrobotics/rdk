// Package ina219 implements an ina219 current/power monitor sensor
// datasheet can be found at: https://www.ti.com/lit/ds/symlink/ina219.pdf
// example repo: https://github.com/esphome/esphome/tree/dev/esphome/components/sht3xd
package ina219

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var modelname = resource.NewDefaultModel("ina219")

const (
	defaultI2Caddr       = 0x40
	senseResistor        = 100 * 1000 * 1000 * 1  // .1 ohm = 100 * milliOhm * microOhm * nanoOhm
	maxCurrent           = 3200 * 1000 * 1000 * 1 // 32 amp = 3200 * milliAmpere * microAmpere * nanoAmpere
	calibratescale       = ((int64(1000*1000*1000*1) * int64(1000*1000*1000*1)) / 100000) << 12
	configRegister       = 0x00
	shuntVoltageRegister = 0x01
	busVoltageRegister   = 0x02
	powerRegister        = 0x03
	currentRegister      = 0x04
	calibrationRegister  = 0x05
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	Board   string `json:"board"`
	I2CBus  string `json:"i2c_bus"`
	I2cAddr int    `json:"i2c_addr,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	if len(config.Board) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, config.Board)
	if len(config.I2CBus) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	return deps, nil
}

func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		modelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attr, ok := config.ConvertedAttributes.(*AttrConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(AttrConfig{}, config.ConvertedAttributes)
			}
			return newSensor(ctx, deps, config.Name, attr, logger)
		}})

	config.RegisterComponentAttributeMapConverter(sensor.Subtype, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newSensor(
	ctx context.Context,
	deps registry.Dependencies,
	name string,
	attr *AttrConfig,
	logger golog.Logger,
) (sensor.Sensor, error) {
	b, err := board.FromDependencies(deps, attr.Board)
	if err != nil {
		return nil, fmt.Errorf("ina219 init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", attr.Board)
	}
	i2cbus, ok := localB.I2CByName(attr.I2CBus)
	if !ok {
		return nil, fmt.Errorf("ina219 init: failed to find i2c bus %s", attr.I2CBus)
	}
	addr := attr.I2cAddr
	if addr == 0 {
		addr = defaultI2Caddr
		logger.Warn("using i2c address : " + string(defaultI2Caddr))
	}

	s := &ina219{
		name:    name,
		logger:  logger,
		bus:     i2cbus,
		busName: attr.I2CBus,
		addr:    byte(addr),
	}

	err = s.calibrate(ctx)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// ina219 is a i2c sensor device that reports voltage, current and power
type ina219 struct {
	generic.Unimplemented
	logger     golog.Logger
	bus        board.I2C
	busName    string
	addr       byte
	name       string
	currentLSB int64
	powerLSB   int64
}

type PowerMonitor struct {
	Shunt   int64
	Voltage int64
	Current int64
	Power   int64
}

func (d *ina219) calibrate(ctx context.Context) error {
	if senseResistor <= 0 {
		return fmt.Errorf("ina219 calibrate: senseResistor value invalid %d", senseResistor)
	}
	if maxCurrent <= 0 {
		return fmt.Errorf("ina219 calibrate: maxCurrent value invalid %d", maxCurrent)
	}

	d.currentLSB = maxCurrent / (1 << 15)
	d.powerLSB = int64((maxCurrent*20 + (1 << 14)) / (1 << 15))
	// Calibration Register = 0.04096 / (current LSB * Shunt Resistance)
	// Where lsb is in Amps and resistance is in ohms.
	// Calibration register is 16 bits.
	cal := calibratescale / (int64(d.currentLSB) * int64(senseResistor))
	if cal >= (1 << 16) {
		return fmt.Errorf("ina219 calibrate: calibration register value invalid %d", cal)
	}
	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		d.logger.Errorf("can't open ina219 i2c %s", err)
		return err
	}
	err = handle.WriteByteData(ctx, calibrationRegister, byte(cal))
	return multierr.Append(err, handle.Close())
}

// Readings returns a list containing three items (voltage, current, and power).
func (d *ina219) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		d.logger.Errorf("can't open ina219 i2c %s", err)
		return nil, err
	}

	var pm PowerMonitor

	shunt, err := handle.ReadByteData(ctx, shuntVoltageRegister)

	if err != nil {
		return nil, err
	}
	// Least significant bit is 10µV.
	pm.Shunt = int64(shunt) * 10 * 1000

	bus, err := handle.ReadByteData(ctx, busVoltageRegister)
	if err != nil {
		return nil, err
	}
	// Check if bit zero is set, if set the ADC has overflowed.
	if binary.BigEndian.Uint16([]byte{0, bus})&1 > 0 {
		return nil, fmt.Errorf("bus voltage register overflow")
	}

	// Least significant bit is 4mV.
	pm.Voltage = int64(binary.BigEndian.Uint16([]byte{0, bus})>>3) * 4 * 1000

	current, err := handle.ReadByteData(ctx, currentRegister)
	if err != nil {
		return nil, err
	}
	pm.Current = int64(current) * d.currentLSB

	power, err := handle.ReadByteData(ctx, powerRegister)
	if err != nil {
		return nil, err
	}
	pm.Power = int64(power) * d.powerLSB

	return map[string]interface{}{
		"voltage":    pm.Voltage,
		"current_mA": pm.Current,
		"power_mW":   pm.Power,
	}, nil
}
