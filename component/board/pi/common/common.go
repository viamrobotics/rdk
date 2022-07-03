// Package picommon contains shared information for supported and non-supported pi boards.
package picommon

import (
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
)

// ModelName is the name used refer to any implementation of a pi based component.
const ModelName = "pi"

// ServoConfig is the config for a pi servo.
type ServoConfig struct {
	Pin      string   `json:"pin"`
	Min      int      `json:"min,omitempty"`
	Max      int      `json:"max,omitempty"`
	StartPos *float64 `json:"starting_position_degrees,omitempty"`
	HoldPos  *bool    `json:"hold_position,omitempty"` // defaults true, holds for 500 ms then disables servo
}

// Validate ensures all parts of the config are valid.
func (config *ServoConfig) Validate(path string) error {
	if config.Pin == "" {
		return utils.NewConfigValidationError(path,
			errors.New("need pin for pi servo"))
	}

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
