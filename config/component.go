package config

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"go.viam.com/core/utils"
)

// A ComponentType defines a type of component.
type ComponentType string

// The set of known component types.
const (
	ComponentTypeBase     = ComponentType("base")
	ComponentTypeArm      = ComponentType("arm")
	ComponentTypeGripper  = ComponentType("gripper")
	ComponentTypeCamera   = ComponentType("camera")
	ComponentTypeLidar    = ComponentType("lidar")
	ComponentTypeSensor   = ComponentType("sensor")
	ComponentTypeProvider = ComponentType("provider")
)

// A Component describes the configuration of a component.
type Component struct {
	Name string `json:"name"`

	Host string `json:"host"`
	Port int    `json:"port"`

	Type    ComponentType `json:"type"`
	SubType string        `json:"subtype"`
	Model   string        `json:"model"`

	Attributes AttributeMap `json:"attributes"`
}

// Ensure Component conforms to flag.Value.
var _ = flag.Value(&Component{})

// String returns a verbose representation of the config.
func (config *Component) String() string {
	return fmt.Sprintf("%#v", config)
}

// Validate ensures all parts of the config are valid.
func (config *Component) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}

// Set hydrates a config based on a flag like value.
func (config *Component) Set(val string) error {
	parsed, err := ParseComponentFlag(val)
	if err != nil {
		return err
	}
	*config = parsed
	return nil
}

// Get gets the config itself.
func (config *Component) Get() interface{} {
	return config
}

// ParseComponentFlag parses a component flag from command line arguments.
func ParseComponentFlag(flag string) (Component, error) {
	cmp := Component{}
	componentParts := strings.Split(flag, ",")
	for _, part := range componentParts {
		keyVal := strings.SplitN(part, "=", 2)
		if len(keyVal) != 2 {
			return Component{}, errors.New("wrong component format; use type=name,host=host,attr=key:value")
		}
		switch keyVal[0] {
		case "name":
			cmp.Name = keyVal[1]
		case "host":
			cmp.Host = keyVal[1]
		case "port":
			port, err := strconv.ParseInt(keyVal[1], 10, 64)
			if err != nil {
				return Component{}, fmt.Errorf("error parsing port: %w", err)
			}
			cmp.Port = int(port)
		case "type":
			cmp.Type = ComponentType(keyVal[1])
		case "subtype":
			cmp.SubType = keyVal[1]
		case "model":
			cmp.Model = keyVal[1]
		case "attr":
			if cmp.Attributes == nil {
				cmp.Attributes = AttributeMap{}
			}
			attrKeyVal := strings.SplitN(keyVal[1], ":", 2)
			if len(attrKeyVal) != 2 {
				return Component{}, errors.New("wrong attribute format; use attr=key:value")
			}
			cmp.Attributes[attrKeyVal[0]] = attrKeyVal[1]
		}
	}
	if string(cmp.Type) == "" {
		return Component{}, errors.New("component type is required")
	}
	return cmp, nil
}
