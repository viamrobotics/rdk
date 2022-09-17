// Package gpsnmea implements an NMEA serial gps.
package gpsnmea

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

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
	SerialPath               string `json:"path"`
	SerialBaudRate           int    `json:"baud_rate,omitempty"`
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
func (config *AttrConfig) Validate(path string) error {
	if config.Board == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	if config.ConnectionType == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	}

	switch config.ConnectionType {
	case i2cStr:
		if config.Board == "" {
			return utils.NewConfigValidationFieldRequiredError(path, "board")
		}
		return config.I2CAttrConfig.ValidateI2C(path)
	case serialStr:
		return config.SerialAttrConfig.ValidateSerial(path)
	default:
		return utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	}
}

// ValidateI2C ensures all parts of the config are valid.
func (config *I2CAttrConfig) ValidateI2C(path string) error {
	if config.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if config.I2cAddr == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}

	return nil
}

// ValidateSerial ensures all parts of the config are valid.
func (config *SerialAttrConfig) ValidateSerial(path string) error {
	if config.SerialPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

const modelname = "gps-nmea"

// NmeaMovementSensor implements a gps that sends nmea messages for movement data
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
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
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
	connectionType := cfg.ConvertedAttributes.(*AttrConfig).ConnectionType

	switch connectionType {
	case serialStr:
		return NewSerialGPSNMEA(ctx, cfg, logger)
	case i2cStr:
		return NewPmtkI2CGPSNMEA(ctx, deps, cfg, logger)
	default:
		return nil, fmt.Errorf("%s is not a valid connection type", connectionType)
	}
}
