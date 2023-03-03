// Package ina219 implements an ina219 current/power monitor sensor
// datasheet can be found at: https://www.ti.com/lit/ds/symlink/ina219.pdf
// example repo: https://github.com/esphome/esphome/tree/dev/esphome/components/sht3xd
package ina219

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/devices/v3/ina219"
	"periph.io/x/host/v3"
)

var modelname = resource.NewDefaultModel("ina219")

const (
	defaultI2Caddr = 0x40
	senseResistor  = 100 * physic.MilliOhm
	maxCurrent     = 3200 * physic.MilliAmpere
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

	s := &ina219Model{
		name:    name,
		logger:  logger,
		bus:     i2cbus,
		busName: attr.I2CBus,
		addr:    byte(addr),
	}

	return s, nil
}

// ina219 is a i2c sensor device that reports voltage, current and power
type ina219Model struct {
	generic.Unimplemented
	logger golog.Logger

	bus     board.I2C
	busName string
	addr    byte
	name    string
}

// Readings returns a list containing three items (voltage, current, and power).
func (s *ina219Model) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	// Make sure periph is initialized.
	if _, err := host.Init(); err != nil {
		return nil, err
	}

	// Open I²C bus.
	bus, err := i2creg.Open(s.busName)
	if err != nil {
		return nil, err
	}
	defer bus.Close()

	// Create a new power sensor.
	sensor, err := ina219.New(bus, &ina219.DefaultOpts)
	if err != nil {
		return nil, err
	}

	// Read values from sensor.
	measurement, err := sensor.Sense()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"voltage":    measurement.Voltage,
		"current_mA": measurement.Current,
		"power_mW":   measurement.Power,
	}, nil
}
