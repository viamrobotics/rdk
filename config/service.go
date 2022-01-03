package config

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"go.viam.com/utils"

	"go.viam.com/rdk/resource"
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

// Ensure Service conforms to flag.Value.
var _ = flag.Value(&Service{})

// String returns a verbose representation of the config.
func (config *Service) String() string {
	return fmt.Sprintf("%#v", config)
}

// ResourceName returns the  ResourceName for the component.
func (config *Service) ResourceName() resource.Name {
	cType := string(config.Type)
	// since services are singletons, the type is sufficient and we don't
	// need an additional name specified in the config. Thus we pass an
	// empty string for that parameter
	return resource.NewName(
		resource.ResourceNamespaceRDK,
		resource.ResourceTypeService,
		resource.SubtypeName(cType),
		"",
	)
}

// Set hydrates a config based on a flag like value.
func (config *Service) Set(val string) error {
	parsed, err := ParseServiceFlag(val)
	if err != nil {
		return err
	}
	*config = parsed
	return nil
}

// Get gets the config itself.
func (config *Service) Get() interface{} {
	return config
}

// ParseServiceFlag parses a service flag from command line arguments.
func ParseServiceFlag(flag string) (Service, error) {
	svc := Service{}
	serviceParts := strings.Split(flag, ",")
	for _, part := range serviceParts {
		keyVal := strings.SplitN(part, "=", 2)
		if len(keyVal) != 2 {
			return Service{}, errors.New("wrong service format; use type=name,attr=key:value")
		}
		switch keyVal[0] {
		case "name":
			svc.Name = keyVal[1]
		case "type":
			svc.Type = ServiceType(keyVal[1])
		case "attr":
			if svc.Attributes == nil {
				svc.Attributes = AttributeMap{}
			}
			attrKeyVal := strings.SplitN(keyVal[1], ":", 2)
			if len(attrKeyVal) != 2 {
				return Service{}, errors.New("wrong attribute format; use attr=key:value")
			}
			svc.Attributes[attrKeyVal[0]] = attrKeyVal[1]
		}
	}
	if string(svc.Type) == "" {
		return Service{}, errors.New("service type is required")
	}
	return svc, nil
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
