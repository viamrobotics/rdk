// Package ina219 implements an ina219 voltage/current/power monitor sensor -
// typically used for battery state monitoring.
// Datasheet can be found at: https://www.ti.com/lit/ds/symlink/ina219.pdf
// Example repo: https://github.com/periph/devices/blob/main/ina219/ina219.go
package ina219

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("power_ina219")

const (
	milliAmp                   = 1000 * 1000 // milliAmp = 1000 microAmpere * 1000 nanoAmpere
	milliOhm                   = 1000 * 1000 // milliOhm = 1000 microOhm * 1000 nanoOhm
	defaultI2Caddr             = 0x40
	senseResistor        int64 = 100 * milliOhm  // .1 ohm
	maxCurrent           int64 = 3200 * milliAmp // 3.2 amp
	calibratescale             = ((int64(1000*milliAmp) * int64(1000*milliOhm)) / 100000) << 12
	configRegister             = 0x00
	shuntVoltageRegister       = 0x01
	busVoltageRegister         = 0x02
	powerRegister              = 0x03
	currentRegister            = 0x04
	calibrationRegister        = 0x05
)

// Config is used for converting config attributes.
type Config struct {
	Board   string `json:"board"`
	I2CBus  string `json:"i2c_bus"`
	I2cAddr int    `json:"i2c_addr,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string
	if len(conf.Board) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, conf.Board)
	if len(conf.I2CBus) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	return deps, nil
}

func init() {
	resource.RegisterComponent(
		sensor.API,
		model,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (sensor.Sensor, error) {
				newConf, err := resource.NativeConfig[*Config](conf)
				if err != nil {
					return nil, err
				}
				return newSensor(deps, conf.ResourceName(), newConf, logger)
			},
		})
}

func newSensor(
	deps resource.Dependencies,
	name resource.Name,
	conf *Config,
	logger golog.Logger,
) (sensor.Sensor, error) {
	b, err := board.FromDependencies(deps, conf.Board)
	if err != nil {
		return nil, fmt.Errorf("ina219 init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", conf.Board)
	}
	i2cbus, ok := localB.I2CByName(conf.I2CBus)
	if !ok {
		return nil, fmt.Errorf("ina219 init: failed to find i2c bus %s", conf.I2CBus)
	}
	addr := conf.I2cAddr
	if addr == 0 {
		addr = defaultI2Caddr
		logger.Infof("using i2c address : %d", defaultI2Caddr)
	}

	s := &ina219{
		Named:  name.AsNamed(),
		logger: logger,
		bus:    i2cbus,
		addr:   byte(addr),
	}

	err = s.calibrate()
	if err != nil {
		return nil, err
	}

	return s, nil
}

// ina219 is a i2c sensor device that reports voltage, current and power.
type ina219 struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger     golog.Logger
	bus        board.I2C
	addr       byte
	currentLSB int64
	powerLSB   int64
	cal        uint16
}

type powerMonitor struct {
	Shunt   int64
	Voltage float64
	Current float64
	Power   float64
}

func (d *ina219) calibrate() error {
	if senseResistor <= 0 {
		return fmt.Errorf("ina219 calibrate: senseResistor value invalid %d", senseResistor)
	}
	if maxCurrent <= 0 {
		return fmt.Errorf("ina219 calibrate: maxCurrent value invalid %d", maxCurrent)
	}

	d.currentLSB = maxCurrent / (1 << 15)
	d.powerLSB = (maxCurrent*20 + (1 << 14)) / (1 << 15)
	// Calibration Register = 0.04096 / (current LSB * Shunt Resistance)
	// Where lsb is in Amps and resistance is in ohms.
	// Calibration register is 16 bits.
	cal := calibratescale / (d.currentLSB * senseResistor)
	if cal >= (1 << 16) {
		return fmt.Errorf("ina219 calibrate: calibration register value invalid %d", cal)
	}
	d.cal = uint16(cal)

	return nil
}

// Readings returns a list containing three items (voltage, current, and power).
func (d *ina219) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		d.logger.Errorf("can't open ina219 i2c: %s", err)
		return nil, err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	// use the calibration result to set the scaling factor
	// of the current and power registers for the maximum resolution
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, d.cal)
	err = handle.WriteBlockData(ctx, calibrationRegister, buf)
	if err != nil {
		return nil, err
	}

	buf = make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(0x1FFF))
	err = handle.WriteBlockData(ctx, configRegister, buf)
	if err != nil {
		return nil, err
	}

	var pm powerMonitor

	// get shunt voltage - currently we are not returning - is it useful?
	shunt, err := handle.ReadBlockData(ctx, shuntVoltageRegister, 2)
	if err != nil {
		return nil, err
	}

	// Least significant bit is 10µV.
	pm.Shunt = int64(binary.BigEndian.Uint16(shunt)) * 10 * 1000
	d.logger.Debugf("ina219 shunt : %d", pm.Shunt)

	bus, err := handle.ReadBlockData(ctx, busVoltageRegister, 2)
	if err != nil {
		return nil, err
	}

	// Check if bit zero is set, if set the ADC has overflowed.
	if binary.BigEndian.Uint16(bus)&1 > 0 {
		return nil, fmt.Errorf("ina219 bus voltage register overflow, register: %d", busVoltageRegister)
	}

	pm.Voltage = float64(binary.BigEndian.Uint16(bus)>>3) * 4 / 1000

	current, err := handle.ReadBlockData(ctx, currentRegister, 2)
	if err != nil {
		return nil, err
	}

	pm.Current = float64(int64(binary.BigEndian.Uint16(current))*d.currentLSB) / 1000000000

	power, err := handle.ReadBlockData(ctx, powerRegister, 2)
	if err != nil {
		return nil, err
	}
	pm.Power = float64(int64(binary.BigEndian.Uint16(power))*d.powerLSB) / 1000000000

	return map[string]interface{}{
		"volts": pm.Voltage,
		"amps":  pm.Current,
		"watts": pm.Power,
	}, nil
}
