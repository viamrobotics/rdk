package api

import (
	"fmt"

	"go.viam.com/robotcore/board"
)

type ComponentType string

const (
	ComponentTypeBase     = ComponentType("base")
	ComponentTypeArm      = ComponentType("arm")
	ComponentTypeGripper  = ComponentType("gripper")
	ComponentTypeCamera   = ComponentType("camera")
	ComponentTypeLidar    = ComponentType("lidar")
	ComponentTypeProvider = ComponentType("provider")
)

type AttributeMap map[string]interface{}

func (am AttributeMap) Has(name string) bool {
	_, has := am[name]
	return has
}

func (am AttributeMap) GetString(name string) string {
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

	Type  ComponentType
	Model string

	Attributes AttributeMap
}

type Remote struct {
	Name    string
	Address string
	Prefix  bool
}

type Config struct {
	Remotes    []Remote
	Boards     []board.Config
	Components []Component
}

func (c Config) FindComponent(name string) *Component {
	for _, cmp := range c.Components {
		if cmp.Name == name {
			return &cmp
		}
	}
	return nil
}
