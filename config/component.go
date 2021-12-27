package config

import (
	"flag"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/resource"
)

// A ComponentType defines a type of component.
type ComponentType string

// The set of known component types.
const (
	ComponentTypeBase            = ComponentType("base")
	ComponentTypeArm             = ComponentType("arm")
	ComponentTypeGantry          = ComponentType("gantry")
	ComponentTypeGripper         = ComponentType("gripper")
	ComponentTypeCamera          = ComponentType("camera")
	ComponentTypeSensor          = ComponentType("sensor")
	ComponentTypeBoard           = ComponentType("board")
	ComponentTypeServo           = ComponentType("servo")
	ComponentTypeMotor           = ComponentType("motor")
	ComponentTypeInputController = ComponentType("input_controller")
)

// A Component describes the configuration of a component.
type Component struct {
	Name string `json:"name"`

	Type      ComponentType `json:"type"`
	SubType   string        `json:"subtype"`
	Model     string        `json:"model"`
	Frame     *Frame        `json:"frame,omitempty"`
	DependsOn []string      `json:"depends_on"`

	Attributes          AttributeMap `json:"attributes"`
	ConvertedAttributes interface{}
}

// Ensure Component conforms to flag.Value.
var _ = flag.Value(&Component{})

// String returns a verbose representation of the config.
func (config *Component) String() string {
	return fmt.Sprintf("%#v", config)
}

// ResourceName returns the  ResourceName for the component.
func (config *Component) ResourceName() resource.Name {
	cType := string(config.Type)
	if config.Type == ComponentTypeSensor {
		cType = config.SubType
	}
	return resource.NewName(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.SubtypeName(cType), config.Name)
}

type validator interface {
	Validate(path string) error
}

// Validate ensures all parts of the config are valid.
func (config *Component) Validate(path string) error {
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
	if v, ok := config.ConvertedAttributes.(validator); ok {
		if err := v.Validate(path); err != nil {
			return err
		}
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
			return Component{}, errors.New("wrong component format; use type=name,host=host,depends_on=name1|name2,attr=key:value")
		}
		switch keyVal[0] {
		case "name":
			cmp.Name = keyVal[1]
		case "type":
			cmp.Type = ComponentType(keyVal[1])
		case "subtype":
			cmp.SubType = keyVal[1]
		case "model":
			cmp.Model = keyVal[1]
		case "depends_on":
			split := strings.Split(keyVal[1], "|")

			var dependsOn []string
			for _, s := range split {
				if s != "" {
					dependsOn = append(dependsOn, s)
				}
			}
			cmp.DependsOn = dependsOn
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
