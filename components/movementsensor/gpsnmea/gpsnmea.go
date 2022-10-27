// Package gpsnmea implements an NMEA serial gps.
package gpsnmea

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

func connectionTypeError(connType, serialConn, i2cConn string) error {
	return errors.Errorf("%s is not a valid connection_type of %s, %s",
		connType,
		serialConn,
		i2cConn)
}

// AttrConfig is used for converting NMEA Movement Sensor attibutes.
type AttrConfig struct {
	ConnectionType string `json:"connection_type"`
	Board          string `json:"board,omitempty"`
	DisableNMEA    bool   `json:"disable_nmea,omitempty"`

	*SerialAttrConfig `json:"serial_attributes,omitempty"`
	*I2CAttrConfig    `json:"i2c_attributes,omitempty"`
}

// SerialAttrConfig is used for converting Serial NMEA MovementSensor config attributes.
type SerialAttrConfig struct {
	SerialPath               string `json:"serial_path"`
	SerialBaudRate           int    `json:"serial_baud_rate,omitempty"`
	SerialCorrectionPath     string `json:"serial_correction_path,omitempty"`
	SerialCorrectionBaudRate int    `json:"serial_correction_baud_rate,omitempty"`
}

// I2CAttrConfig is used for converting Serial NMEA MovementSensor config attributes.
type I2CAttrConfig struct {
	I2CBus      string `json:"i2c_bus"`
	I2cAddr     int    `json:"i2c_addr"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.Board == "" && cfg.ConnectionType == i2cStr {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	if cfg.ConnectionType == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	}

	switch cfg.ConnectionType {
	case i2cStr:
		if cfg.Board == "" {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
		}
		deps = append(deps, cfg.Board)
		return deps, cfg.I2CAttrConfig.ValidateI2C(path)
	case serialStr:
		return nil, cfg.SerialAttrConfig.ValidateSerial(path)
	default:
		return nil, utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	}
}

// ValidateI2C ensures all parts of the config are valid.
func (cfg *I2CAttrConfig) ValidateI2C(path string) error {
	if cfg.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2cAddr == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}

	return nil
}

// ValidateSerial ensures all parts of the config are valid.
func (cfg *SerialAttrConfig) ValidateSerial(path string) error {
	if cfg.SerialPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

const modelname = "gps-nmea"

// NmeaMovementSensor implements a gps that sends nmea messages for movement data.
type NmeaMovementSensor interface {
	movementsensor.MovementSensor
	Start(ctx context.Context) error          // Initialize and run MovementSensor
	Close() error                             // Close MovementSensor
	ReadFix(ctx context.Context) (int, error) // Returns the fix quality of the current MovementSensor measurements
}

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		modelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			cfg config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newNMEAGPS(ctx, deps, cfg, logger)
		}})

	config.RegisterComponentAttributeMapConverter(movementsensor.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr AttrConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		},
		&AttrConfig{})
}

const (
	connectionType = "connection_type"
	i2cStr         = "I2C"
	serialStr      = "serial"
	rtkStr         = "rtk"
)

func newNMEAGPS(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	attr, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, cfg.ConvertedAttributes)
	}

	switch attr.ConnectionType {
	case serialStr:
		return NewSerialGPSNMEA(ctx, attr, logger)
	case i2cStr:
		return NewPmtkI2CGPSNMEA(ctx, deps, attr, logger)
	default:
		return nil, connectionTypeError(
			attr.ConnectionType,
			i2cStr,
			serialStr)
	}
}
