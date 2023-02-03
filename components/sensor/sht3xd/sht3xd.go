// Package sht3xd implements a sht3x-d sensor for temperature and humidity
package sht3xd

import (
	"context"
	"encoding/binary"
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
)

var modelname = resource.NewDefaultModel("sht3xd")

const (
	defaultI2Caddr = 0x44

	// Addresses of sht3xd registers.

	sht3xdCOMMANDREADSERIALNUMBER = 0x3780;
	sht3xdCOMMANDREADSTATUS = 0xF32D;
	sht3xdCOMMANDCLEARSTATUS = 0x3041;
	sht3xdCOMMANDHEATERENABLE = 0x306D;
	sht3xdCOMMANDHEATERDISABLE = 0x3066;
	sht3xdCOMMANDSOFTRESET = 0x30A2;
	sht3xdCOMMANDPOLLINGH = 0x2400;
	sht3xdCOMMANDFETCHDATA = 0xE000;
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
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c bus")
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
		return nil, fmt.Errorf("sht3xd init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", attr.Board)
	}
	i2cbus, ok := localB.I2CByName(attr.I2CBus)
	if !ok {
		return nil, fmt.Errorf("sht3xd init: failed to find i2c bus %s", attr.I2CBus)
	}
	addr := attr.I2cAddr
	if addr == 0 {
		addr = defaultI2Caddr
		logger.Warn("using i2c address :", defaultI2Caddr)
	}

	s := &sht3xd{
		name:     name,
		logger:   logger,
		bus:      i2cbus,
		addr:     byte(addr),
	}
	
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		s.logger.Errorf("can't open sht3xd i2c %s", err)
		return nil, err
	}
	err = handle.Write(ctx, []byte{0x30,0xA2})
	if err != nil {
		s.logger.Debug("Failed to soft reset")
	}

	return s, handle.Close()
}

// sht3xd is a i2c sensor device.
type sht3xd struct {
	generic.Unimplemented
	logger golog.Logger

	bus         board.I2C
	addr        byte
	name        string
}

// Readings returns a list containing single item (current temperature).
func (s *sht3xd) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		s.logger.Errorf("can't open sht3xd i2c %s", err)
		return nil, err
	}
	err = handle.Write(ctx, []byte{0x24,0x00})
	if err != nil {
		s.logger.Debug("Failed to request temperature")
	}
	buffer, err := handle.Read(ctx, 2)
	if err != nil {
		return nil, err
	}
	tempRaw := binary.LittleEndian.Uint16([]byte{0, buffer[0]})
	humidRaw := binary.LittleEndian.Uint16([]byte{0, buffer[1]})

	temp := 175.0 * float64(tempRaw) / 65535.0 - 45.0
	humid := 100.0 * float64(humidRaw) / 65535.0
	return map[string]interface{}{
		"temperature_celsius":    temp,
		"temperature_fahrenheit": temp*1.8 + 32,
		"humidity":        humid,
	}, handle.Close()
}
