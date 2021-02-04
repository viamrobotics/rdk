package robot

import (
	"encoding/json"
	"os"

	"github.com/edaniels/golog"
)

type ComponentType string

const (
	ComponentTypeBase    = ComponentType("base")
	ComponentTypeArm     = ComponentType("arm")
	ComponentTypeGripper = ComponentType("gripper")
	ComponentTypeCamera  = ComponentType("camera")
	ComponentTypeLidar   = ComponentType("lidar")
)

type Component struct {
	Name string

	Host string
	Port int

	Type  ComponentType
	Model string

	Attributes map[string]string
}

type Config struct {
	Components []Component
	Logger     golog.Logger
}

func ReadConfig(fn string) (Config, error) {
	cfg := Config{}

	file, err := os.Open(fn)
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		return cfg, err
	}

	for idx, c := range cfg.Components {
		for k, v := range c.Attributes {
			cfg.Components[idx].Attributes[k] = os.ExpandEnv(v)
		}
	}

	return cfg, nil
}
