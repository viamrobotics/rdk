package robot

import (
	"encoding/json"
	"os"

	"github.com/echolabsinc/robotcore/utils/log"
)

type ComponentType string

const (
	Arm     = "arm"
	Gripper = "gripper"
	Camera  = "camera"
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
	Logger     log.Logger
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
