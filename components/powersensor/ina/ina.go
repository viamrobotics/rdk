//go:build linux

// Package ina implements ina power sensors to measure voltage, current, and power
// INA219 datasheet: https://www.ti.com/lit/ds/symlink/ina219.pdf
// Example repo: https://github.com/periph/devices/blob/main/ina219/ina219.go
// INA226 datasheet: https://www.ti.com/lit/ds/symlink/ina226.pdf

// The voltage, current and power can be read as
// 16 bit big endian integers from their given registers.
// This value is multiplied by the register LSB to get the reading in nanounits.

// Voltage LSB: 1.25 mV for INA226, 4 mV for INA219
// Current LSB: maximum expected current of the system / (1 << 15)
// Power LSB: 25*CurrentLSB for INA226, 20*CurrentLSB for INA219

// The calibration register is programmed to measure current and power properly.
// The calibration register is set to: calibratescale / (currentLSB * senseResistor)

package ina

import (
	"context"
	"errors"
	"fmt"

	"github.com/d2r2/go-i2c"
	i2clog "github.com/d2r2/go-logger"
	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

const (
	modelName219         = "ina219"
	modelName226         = "ina226"
	defaultI2Caddr       = 0x40
	configRegister       = 0x00
	shuntVoltageRegister = 0x01
	busVoltageRegister   = 0x02
	powerRegister        = 0x03
	currentRegister      = 0x04
	calibrationRegister  = 0x05
)

// values for inas in nano units so need to convert.
var (
	senseResistor = toNano(0.1) // .1 ohm
	maxCurrent219 = toNano(3.2) // 3.2 amp
	maxCurrent226 = toNano(20)  // 20 amp
)

// need to scale, making sure to not overflow int64.
var (
	calibratescale219 = (toNano(1) * toNano(1) / 100000) << 12 // .04096 is internal fixed value for ina219
	calibrateScale226 = (toNano(1) * toNano(1) / 100000) << 9  // .00512 is internal fixed value for ina226
)

var inaModels = []string{modelName219, modelName226}

// Config is used for converting config attributes.
type Config struct {
	I2CBus          int     `json:"i2c_bus"`
	I2cAddr         int     `json:"i2c_addr,omitempty"`
	MaxCurrent      float64 `json:"max_current_amps,omitempty"`
	ShuntResistance float64 `json:"shunt_resistance,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string
	if conf.I2CBus == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	return deps, nil
}

func init() {
	for _, modelName := range inaModels {
		localModelName := modelName
		inaModel := resource.DefaultModelFamily.WithModel(modelName)
		resource.RegisterComponent(
			powersensor.API,
			inaModel,
			resource.Registration[powersensor.PowerSensor, *Config]{
				Constructor: func(
					ctx context.Context,
					deps resource.Dependencies,
					conf resource.Config,
					logger golog.Logger,
				) (powersensor.PowerSensor, error) {
					newConf, err := resource.NativeConfig[*Config](conf)
					if err != nil {
						return nil, err
					}
					return newINA(conf.ResourceName(), newConf, logger, localModelName)
				},
			})
	}
}

func newINA(
	name resource.Name,
	conf *Config,
	logger golog.Logger,
	modelName string,
) (powersensor.PowerSensor, error) {
	err := i2clog.ChangePackageLogLevel("i2c", i2clog.InfoLevel)
	if err != nil {
		return nil, err
	}

	addr := conf.I2cAddr
	if addr == 0 {
		addr = defaultI2Caddr
		logger.Infof("using i2c address : %d", defaultI2Caddr)
	}

	maxCurrent := toNano(conf.MaxCurrent)
	if maxCurrent == 0 {
		switch modelName {
		case modelName219:
			maxCurrent = maxCurrent219
			logger.Info("using default max current 3.2A")
		case modelName226:
			maxCurrent = maxCurrent226
			logger.Info("using default max current 20A")
		}
	}

	resistance := toNano(conf.ShuntResistance)
	if resistance == 0 {
		resistance = senseResistor
		logger.Info("using default resistor value 0.1 ohms")
	}

	s := &ina{
		Named:      name.AsNamed(),
		logger:     logger,
		model:      modelName,
		bus:        conf.I2CBus,
		addr:       byte(addr),
		maxCurrent: maxCurrent,
		resistance: resistance,
	}

	err = s.setCalibrationScale(modelName)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// ina is a i2c sensor device that reports voltage, current and power.
type ina struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger     golog.Logger
	model      string
	bus        int
	addr       byte
	currentLSB int64
	powerLSB   int64
	cal        uint16
	maxCurrent int64
	resistance int64
}

func (d *ina) setCalibrationScale(modelName string) error {
	var calibratescale int64
	d.currentLSB = d.maxCurrent / (1 << 15)
	switch modelName {
	case modelName219:
		calibratescale = calibratescale219
		d.powerLSB = (d.maxCurrent*20 + (1 << 14)) / (1 << 15)
	case modelName226:
		calibratescale = calibrateScale226
		d.powerLSB = 25 * d.currentLSB
	default:
		return errors.New("ina model not supported")
	}

	// Calibration Register = calibration scale / (current LSB * Shunt Resistance)
	// Where lsb is in Amps and resistance is in ohms.
	// Calibration register is 16 bits.
	cal := calibratescale / (d.currentLSB * d.resistance)
	if cal >= (1 << 16) {
		return fmt.Errorf("ina calibrate: calibration register value invalid %d", cal)
	}
	d.cal = uint16(cal)

	return nil
}

func (d *ina) calibrate() error {
	handle, err := i2c.NewI2C(d.addr, d.bus)
	if err != nil {
		d.logger.Errorf("can't open ina i2c: %s", err)
		return err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	// use the calibration result to set the scaling factor
	// of the current and power registers for the maximum resolution
	err = handle.WriteRegU16BE(calibrationRegister, d.cal)
	if err != nil {
		return err
	}

	// setting config to 111 sets to normal operating mode
	err = handle.WriteRegU16BE(configRegister, uint16(0x399F))
	if err != nil {
		return err
	}
	return nil
}

func (d *ina) Voltage(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	handle, err := i2c.NewI2C(d.addr, d.bus)
	if err != nil {
		d.logger.Errorf("can't open ina i2c: %s", err)
		return 0, false, err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	bus, err := handle.ReadRegS16BE(busVoltageRegister)
	if err != nil {
		return 0, false, err
	}

	var voltage float64
	switch d.model {
	case modelName226:
		// voltage is 1.25 mV/bit for the ina226
		voltage = float64(bus) * 1.25e-3
	case modelName219:
		// lsb is 4mV, must shift right 3 bits
		voltage = float64(bus>>3) * 4 / 1000
	default:
	}

	isAC := false
	return voltage, isAC, nil
}

func (d *ina) Current(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	handle, err := i2c.NewI2C(d.addr, d.bus)
	if err != nil {
		d.logger.Errorf("can't open ina i2c: %s", err)
		return 0, false, err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	// Calibrate each time the current value is read, so if anything else is also writing to these registers
	// we have the correct value.
	err = d.calibrate()
	if err != nil {
		return 0, false, err
	}

	rawCur, err := handle.ReadRegS16BE(currentRegister)
	if err != nil {
		return 0, false, err
	}

	current := fromNano(float64(int64(rawCur) * d.currentLSB))
	isAC := false
	return current, isAC, nil
}

func (d *ina) Power(ctx context.Context, extra map[string]interface{}) (float64, error) {
	handle, err := i2c.NewI2C(d.addr, d.bus)
	if err != nil {
		d.logger.Errorf("can't open ina i2c handle: %s", err)
		return 0, err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	// Calibrate each time the power value is read, so if anything else is also writing to these registers
	// we have the correct value.
	err = d.calibrate()
	if err != nil {
		return 0, err
	}

	pow, err := handle.ReadRegS16BE(powerRegister)
	if err != nil {
		return 0, err
	}
	power := fromNano(float64(int64(pow) * d.powerLSB))
	return power, nil
}

// Readings returns a map with voltage, current, power and isAC.
func (d *ina) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	volts, isAC, err := d.Voltage(ctx, nil)
	if err != nil {
		d.logger.Errorf("failed to get voltage reading: %s", err.Error())
	}

	amps, _, err := d.Current(ctx, nil)
	if err != nil {
		d.logger.Errorf("failed to get current reading: %s", err.Error())
	}

	watts, err := d.Power(ctx, nil)
	if err != nil {
		d.logger.Errorf("failed to get power reading: %s", err.Error())
	}
	return map[string]interface{}{
		"volts": volts,
		"amps":  amps,
		"is_ac": isAC,
		"watts": watts,
	}, nil
}

func toNano(value float64) int64 {
	nano := value * 1e9
	return int64(nano)
}

func fromNano(value float64) float64 {
	unit := value / 1e9
	return unit
}
