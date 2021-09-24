package functionvm

import (
	"fmt"

	"go.viam.com/utils"
)

// Function is a generic function that can be called across engines.
type Function func(args ...Value) ([]Value, error)

// An AnonymousFunctionConfig defines a unnamed function.
type AnonymousFunctionConfig struct {
	Engine EngineName `json:"engine"`
	Source string     `json:"source"`
}

// Validate ensures all parts of the config are valid.
func (config *AnonymousFunctionConfig) Validate(path string) error {
	if config.Engine == EngineName("") {
		return utils.NewConfigValidationFieldRequiredError(path, "engine")
	}
	if config.Source == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "source")
	}
	if err := ValidateSource(config.Engine, config.Source); err != nil {
		return utils.NewConfigValidationError(fmt.Sprintf("%s.source", path), err)
	}
	return nil
}

// A FunctionConfig defines a function.
type FunctionConfig struct {
	AnonymousFunctionConfig
	Name string `json:"name"`
}

// Validate ensures all parts of the config are valid.
func (config *FunctionConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return config.AnonymousFunctionConfig.Validate(path)
}
