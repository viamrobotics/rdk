package picommon

import (
	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
)

// ServoConfig is the config for a pi servo.
type ServoConfig struct {
	Pin         string   `json:"pin"`
	Min         int      `json:"min,omitempty"`
	Max         int      `json:"max,omitempty"` // specifies a user inputted position limitation
	StartPos    *float64 `json:"starting_position_degs,omitempty"`
	HoldPos     *bool    `json:"hold_position,omitempty"` // defaults True. False holds for 500 ms then disables servo
	BoardName   string   `json:"board"`
	MaxRotation int      `json:"max_rotation_deg,omitempty"` // specifies a hardware position limitation. Defaults to 180
}

// Validate ensures all parts of the config are valid.
func (config *ServoConfig) Validate(path string) ([]string, error) {
	var deps []string
	if config.Pin == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("need pin for pi servo"))
	}
	if config.BoardName == "" {
		return nil, resource.NewConfigValidationError(path,
			errors.New("need the name of the board"))
	}
	deps = append(deps, config.BoardName)
	return deps, nil
}
