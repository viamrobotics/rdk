package config

import (
	"fmt"

	"go.viam.com/utils"
)

// A ServiceType defines a type of service.
type ServiceType string

// A Service describes the configuration of a service.
type Service struct {
	Name                string       `json:"name"`
	Type                ServiceType  `json:"type"`
	Attributes          AttributeMap `json:"attributes"`
	ConvertedAttributes interface{}
}

// Validate ensures all parts of the config are valid.
func (config *Service) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	for key, value := range config.Attributes {
		v, ok := value.(validator)
		if !ok {
			continue
		}
		if err := v.Validate(fmt.Sprintf("%s.%s", path, key)); err != nil {
			return err
		}
	}
	return nil
}
