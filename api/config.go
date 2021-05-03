package api

import (
	"fmt"
	"time"

	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/rexec"
	"go.viam.com/robotcore/utils"
)

// A Config describes the configuration of a robot.
type Config struct {
	ConfigFilePath string
	Cloud          *CloudConfig          `json:"cloud,omitempty"`
	Remotes        []RemoteConfig        `json:"remotes,omitempty"`
	Boards         []board.Config        `json:"boards,omitempty"`
	Components     []ComponentConfig     `json:"components,omitempty"`
	Processes      []rexec.ProcessConfig `json:"processes,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (c Config) Validate() error {
	if c.Cloud != nil {
		if err := c.Cloud.Validate("cloud"); err != nil {
			return err
		}
	}

	for idx, config := range c.Remotes {
		if err := config.Validate(fmt.Sprintf("%s.%d", "remotes", idx)); err != nil {
			return err
		}
	}

	for idx, config := range c.Boards {
		if err := config.Validate(fmt.Sprintf("%s.%d", "boards", idx)); err != nil {
			return err
		}
	}

	for idx, config := range c.Components {
		if err := config.Validate(fmt.Sprintf("%s.%d", "components", idx)); err != nil {
			return err
		}
	}

	for idx, config := range c.Processes {
		if err := config.Validate(fmt.Sprintf("%s.%d", "processes", idx)); err != nil {
			return err
		}
	}

	return nil
}

// FindComponent finds a particular component by name.
func (c Config) FindComponent(name string) *ComponentConfig {
	for _, cmp := range c.Components {
		if cmp.Name == name {
			return &cmp
		}
	}
	return nil
}

// A RemoteConfig describes a remote robot that should be integrated.
type RemoteConfig struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Prefix  bool   `json:"prefix"`
}

// Validate ensures all parts of the config are valid.
func (config *RemoteConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	if config.Address == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "address")
	}
	return nil
}

// A CloudConfig describes how to configure a robot controlled by the
// cloud.
// The cloud source could be anything that supports http.
// URL is constructed as $Path?id=ID and secret is put in a http header.
type CloudConfig struct {
	ID              string        `json:"id"`
	Secret          string        `json:"secret"`
	Path            string        `json:"path,omitempty"`    // optional, defaults to viam cloud otherwise
	LogPath         string        `json:"logPath,omitempty"` // optional, defaults to viam cloud otherwise
	RefreshInterval time.Duration `json:"refresh_interval,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *CloudConfig) Validate(path string) error {
	if config.ID == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "id")
	}
	if config.Secret == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "secret")
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 10 * time.Second
	}
	return nil
}
