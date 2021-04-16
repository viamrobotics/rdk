package api

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.viam.com/robotcore/board"
)

type ComponentType string

const (
	ComponentTypeBase     = ComponentType("base")
	ComponentTypeArm      = ComponentType("arm")
	ComponentTypeGripper  = ComponentType("gripper")
	ComponentTypeCamera   = ComponentType("camera")
	ComponentTypeLidar    = ComponentType("lidar")
	ComponentTypeSensor   = ComponentType("sensor")
	ComponentTypeProvider = ComponentType("provider")
)

type AttributeMap map[string]interface{}

func (am AttributeMap) Has(name string) bool {
	_, has := am[name]
	return has
}

func (am AttributeMap) GetString(name string) string {
	if am == nil {
		return ""
	}
	x := am[name]
	if x == nil {
		return ""
	}

	s, ok := x.(string)
	if ok {
		return s
	}

	panic(fmt.Errorf("wanted a string for (%s) but got (%v) %T", name, x, x))
}

func (am AttributeMap) GetInt(name string, def int) int {
	if am == nil {
		return def
	}
	x, has := am[name]
	if !has {
		return def
	}

	v, ok := x.(int)
	if ok {
		return v
	}

	v2, ok := x.(float64)
	if ok {
		// TODO(erh): is this safe? json defaults to float64, so seems nice
		return int(v2)
	}

	panic(fmt.Errorf("wanted an int for (%s) but got (%v) %T", name, x, x))
}

func (am AttributeMap) GetFloat64(name string, def float64) float64 {
	if am == nil {
		return def
	}
	x, has := am[name]
	if !has {
		return def
	}

	v, ok := x.(float64)
	if ok {
		return v
	}

	panic(fmt.Errorf("wanted an int for (%s) but got (%v) %T", name, x, x))
}

func (am AttributeMap) GetBool(name string, def bool) bool {
	if am == nil {
		return def
	}
	x, has := am[name]
	if !has {
		return def
	}

	v, ok := x.(bool)
	if ok {
		return v
	}

	panic(fmt.Errorf("wanted a bool for (%s) but got (%v) %T", name, x, x))
}

type Component struct {
	Name string

	Host string
	Port int

	Type    ComponentType
	SubType string
	Model   string

	Attributes AttributeMap
}

func (desc *Component) String() string {
	return fmt.Sprintf("%#v", desc)
}

func (desc *Component) Set(val string) error {
	parsed, err := ParseComponentFlag(val)
	if err != nil {
		return err
	}
	*desc = parsed
	return nil
}

func (desc *Component) Get() interface{} {
	return desc
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

type Remote struct {
	Name    string
	Address string
	Prefix  bool
}

// configuration for how to fetch the actual config from a cloud source
// the cloud source could be anything that supports http
// url is constructed as $Path?id=ID and secret is put in a http header
type CloudConfig struct {
	ID      string
	Secret  string
	Path    string // optional, defaults to viam cloud otherwise
	LogPath string // optional, defaults to viam cloud otherwise
}

type Config struct {
	Remotes    []Remote
	Boards     []board.Config
	Components []Component
	Cloud      CloudConfig
}

func (c Config) FindComponent(name string) *Component {
	for _, cmp := range c.Components {
		if cmp.Name == name {
			return &cmp
		}
	}
	return nil
}
