// Package picommon contains shared information for supported and non-supported pi boards.
package picommon

import (
	"errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/utils"
)

// ModelName is the name used refer to any implementation of a pi based component.
const ModelName = "pi"

// ServoConfig is the config for a pi servo.
type ServoConfig struct {
	Pin      string   `json:"pin"`
	Min      int      `json:"min,omitempty"`
	Max      int      `json:"max,omitempty"`
	StartPos *float64 `json:"starting_position_degrees,omitempty"`
	HoldPos  *bool    `json:"hold_position,omitempty"` // defaults true, holds servo position for 500 ms then disables motor when false. For safety.
}

// Validate ensures all parts of the config are valid.
func (config *ServoConfig) Validate(path string) error {

	if config.Pin == "" {
		utils.NewConfigValidationError(path, errors.New("board pin is unspecified for servo"))
	}

	// no other attribute is required

	return nil
}

func init() {
	board.RegisterConfigAttributeConverter(ModelName)

	config.RegisterComponentAttributeMapConverter(
		servo.SubtypeName,
		ModelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf ServoConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&ServoConfig{})
}
