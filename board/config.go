package board

import (
	"fmt"

	"go.viam.com/core/utils"
)

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	Name              string                   `json:"name"`
	Model             string                   `json:"model"` // example: "pi"
	Motors            []MotorConfig            `json:"motors"`
	Servos            []ServoConfig            `json:"servos"`
	Analogs           []AnalogConfig           `json:"analogs"`
	DigitalInterrupts []DigitalInterruptConfig `json:"digitalInterrupts"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	for idx, conf := range config.Motors {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "motors", idx)); err != nil {
			return err
		}
	}
	for idx, conf := range config.Servos {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "servos", idx)); err != nil {
			return err
		}
	}
	for idx, conf := range config.Analogs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return err
		}
	}
	for idx, conf := range config.DigitalInterrupts {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "digital_interrupts", idx)); err != nil {
			return err
		}
	}
	return nil
}

// MotorConfig describes the configuration of a motor on a board.
type MotorConfig struct {
	Name             string            `json:"name"`
	Pins             map[string]string `json:"pins"`
	Encoder          string            `json:"encoder"`  // name of the digital interrupt that is the encoder
	EncoderB         string            `json:"encoderB"` // name of the digital interrupt that is hall encoder b
	TicksPerRotation int               `json:"ticksPerRotation"`
}

// Validate ensures all parts of the config are valid.
func (config *MotorConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}

// ServoConfig describes the configuration of a servo on a board.
type ServoConfig struct {
	Name string `json:"name"`
	Pin  string `json:"pin"`
}

// Validate ensures all parts of the config are valid.
func (config *ServoConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}

// AnalogConfig describes the configuration of an analog reader on a board.
type AnalogConfig struct {
	Name              string `json:"name"`
	Pin               string `json:"pin"`
	AverageOverMillis int    `json:"averageOverMillis"`
	SamplesPerSecond  int    `json:"samplesPerSecond"`
}

// Validate ensures all parts of the config are valid.
func (config *AnalogConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}

// DigitalInterruptConfig describes the configuration of digital interrupt for a board.
type DigitalInterruptConfig struct {
	Name    string `json:"name"`
	Pin     string `json:"pin"`
	Type    string `json:"type"` // e.g. basic, servo
	Formula string `json:"formula"`
}

// Validate ensures all parts of the config are valid.
func (config *DigitalInterruptConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}
