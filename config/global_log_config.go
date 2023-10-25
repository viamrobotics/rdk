package config

import (
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// GlobalLogConfig represents log declarations for all APIs or Models of a certain variety on a
// robot.
type GlobalLogConfig struct {
	API resource.API `json:"api"`
	// The `Model` is a pointer such that "omitempty" skips over the field when nil.
	Model *resource.Model `json:"model,omitempty"`
	Level logging.Level   `json:"level"`
}

// Validate the GlobalLogConfig.
func (glc GlobalLogConfig) Validate(path string) error {
	if err := glc.API.Validate(); err != nil {
		return utils.NewConfigValidationError(path, err)
	}

	if glc.Model == nil {
		return nil
	}

	if err := glc.Model.Validate(); err != nil {
		return utils.NewConfigValidationError(path, err)
	}

	return nil
}
