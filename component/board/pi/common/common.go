// Package picommon contains shared information for supported and non-supported pi boards.
package picommon

import (
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
)

// ModelName is the name used refer to any implementation of a pi based component.
const ModelName = "pi"

// ServoConfig is the config for a pi servo.
type ServoConfig struct {
	Pin string `json:"pin"`
	Min int    `json:"min,omitempty"`
	Max int    `json:"max,omitempty"`
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
