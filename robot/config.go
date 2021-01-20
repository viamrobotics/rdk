package robot

import (
	"encoding/json"
	"os"

	"github.com/edaniels/golog"
)

type ComponentType string

const (
	Arm     = "arm"
	Gripper = "gripper"
	Camera  = "camera"
	Lidar   = "lidar"
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

	return cfg, nil
}
