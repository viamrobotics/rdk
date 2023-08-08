// Package ina implements ina power sensors
// typically used for battery state monitoring.
// INA219 datasheet: https://www.ti.com/lit/ds/symlink/ina219.pdf
// Example repo: https://github.com/periph/devices/blob/main/ina219/ina219.go
// INA226 datasheet:
package ina

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

const (
	modelName219               = "ina219"
	modelName226               = "ina226"
	milliAmp                   = 1000 * 1000 // milliAmp = 1000 microAmpere * 1000 nanoAmpere
	milliOhm                   = 1000 * 1000 // milliOhm = 1000 microOhm * 1000 nanoOhm
	defaultI2Caddr             = 0x40
	senseResistor        int64 = 100 * milliOhm                                                 // .1 ohm
	maxCurrent           int64 = 20000 * milliAmp                                               // 3.2 amp
	calibratescale219          = ((int64(1000*milliAmp) * int64(1000*milliOhm)) / 100000) << 12 //.04096 is internal fixed value for ina219
	calibrateScale226          = ((int64(1000*milliAmp) * int64(1000*milliOhm)) / 100000) << 9  //.00512 is internal fixed value for ina226
	configRegister             = 0x00
	shuntVoltageRegister       = 0x01
	busVoltageRegister         = 0x02
	powerRegister              = 0x03
	currentRegister            = 0x04
	calibrationRegister        = 0x05
)

var inaModels = []string{modelName219, modelName226}

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
					return newINA(ctx, deps, conf.ResourceName(), newConf, logger, localModelName)
				},
			})
	}
}

func newINA(
	ctx context.Context,
	deps resource.Dependencies,
	name resource.Name,
	conf *Config,
	logger golog.Logger,
	modelName string,
) (powersensor.PowerSensor, error) {

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

	s := &ina{
		Named:  name.AsNamed(),
		logger: logger,
		model:  modelName,
		bus:    i2cbus,
		addr:   byte(addr),
	}

	err = s.setCalibrationScale(modelName)
	if err != nil {
		return nil, err
	}

	err = s.calibrate(ctx)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// ina219 is a i2c sensor device that reports voltage, current and power.
type ina struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger     golog.Logger
	model      string
	bus        board.I2C
	addr       byte
	currentLSB int64
	powerLSB   int64
	cal        uint16
}

func (d *ina) setCalibrationScale(modelName string) error {
	var calibratescale int64
	if senseResistor <= 0 {
		return fmt.Errorf("ina219 calibrate: senseResistor value invalid %d", senseResistor)
	}
	if maxCurrent <= 0 {
		return fmt.Errorf("ina219 calibrate: maxCurrent value invalid %d", maxCurrent)
	}

	d.currentLSB = maxCurrent / (1 << 15)

	switch modelName {
	case modelName219:
		calibratescale = calibratescale219
		d.powerLSB = (maxCurrent*20 + (1 << 14)) / (1 << 15)
	case modelName226:
		calibratescale = calibrateScale226
		d.powerLSB = 25 * d.currentLSB
	default:
		return errors.New("ina model not supported")
	}

	// Calibration Register = calibration scale / (current LSB * Shunt Resistance)
	// Where lsb is in Amps and resistance is in ohms.
	// Calibration register is 16 bits.
	cal := calibratescale / (d.currentLSB * senseResistor)
	if cal >= (1 << 16) {
		return fmt.Errorf("ina219 calibrate: calibration register value invalid %d", cal)
	}
	d.cal = uint16(cal)

	return nil
}

func (d *ina) calibrate(ctx context.Context) error {
	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		d.logger.Errorf("can't open ina219 i2c: %s", err)
		return err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	// use the calibration result to set the scaling factor
	// of the current and power registers for the maximum resolution
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, d.cal)
	err = handle.WriteBlockData(ctx, calibrationRegister, buf)
	if err != nil {
		return err
	}

	// setting config to 111 sets to normal operating mode
	buf = make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(0x6F))
	err = handle.WriteBlockData(ctx, configRegister, buf)
	if err != nil {
		return err
	}
	return nil
}

func (d *ina) Voltage(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		d.logger.Errorf("can't open ina i2c: %s", err)
		return 0, false, err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	bus, err := handle.ReadBlockData(ctx, busVoltageRegister, 2)
	if err != nil {
		return 0, false, err
	}

	var voltage float64
	switch d.model {
	case modelName226:
		// voltage is 1.25 mV/bit for the ina226
		voltage = float64(binary.BigEndian.Uint16(bus)) * 1.25e-3
	case modelName219:
		voltage = float64(binary.BigEndian.Uint16(bus)>>3) * 4 / 1000
	default:
	}

	isAC := false
	return voltage, isAC, nil
}

func (d *ina) Current(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		d.logger.Errorf("can't open ina i2c: %s", err)
		return 0, false, err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	rawCur, err := handle.ReadBlockData(ctx, currentRegister, 2)
	if err != nil {
		return 0, false, err
	}

	current := float64(int64(binary.BigEndian.Uint16(rawCur))*d.currentLSB) / 1000000000
	isAC := false
	return current, isAC, nil
}

func (d *ina) Power(ctx context.Context, extra map[string]interface{}) (float64, error) {
	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		d.logger.Errorf("can't open ina i2c handle: %s", err)
		return 0, err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	pow, err := handle.ReadBlockData(ctx, powerRegister, 2)
	if err != nil {
		return 0, err
	}
	power := float64(int64(binary.BigEndian.Uint16(pow))*d.powerLSB) / 1000000000
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
