package robot

import (
	"encoding/json"
	"os"
)

type ComponentType string

const (
	Arm     = "arm"
	Gripper = "gripper"
	Camera  = "camera"
)

type Component struct {
	Host string
	Port int

	Type  ComponentType
	Model string
}

type Config struct {
	Components []Component
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
