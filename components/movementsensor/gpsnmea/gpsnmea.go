// Package nmea implements an NMEA serial gps.
package gpsnmea

import (
	"context"
	"errors"
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
	Board          string `json:"board"`
	DisableNMEA    bool   `json:"disable_nmea,omitempty"`

	*SerialAttrConfig

	*I2CAttrConfig
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	if config.Board == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	if config.ConnectionType == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	}

	if config.SerialAttrConfig != nil {
		return config.SerialAttrConfig.ValidateSerial(path)
	}

	if config.I2CAttrConfig != nil {
		return config.I2CAttrConfig.ValidateI2C(path)
	}

	// if config.RTKAttrConfig != nil {
	// 	return config.RTKAttrConfig.ValidateRTK(path)
	// }

	if config == nil {
		return utils.NewConfigValidationError(path, errors.New("no config found"))
	}

	return nil
}

const modelname = "gps-nmea"

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
			return NewNMEAGPS(ctx, deps, cfg, logger)
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

func NewNMEAGPS(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	connectionType := cfg.ConvertedAttributes.(*AttrConfig).ConnectionType

	switch connectionType {
	case serialStr:
		return NewSerialNMEAMovementSensor(ctx, cfg, logger)
	case i2cStr:
		return NewPmtkI2CNMEAMovementSensor(ctx, deps, cfg, logger)
	default:
		return nil, fmt.Errorf("%s is not a valid connection type", connectionType)
	}
}
