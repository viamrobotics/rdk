// Package config defines the structures to configure a robot and its connected parts.
package config

import (
	"fmt"
	"time"

	"go.viam.com/core/board"
	"go.viam.com/core/rexec"
	"go.viam.com/core/utils"
)

// A Config describes the configuration of a robot.
type Config struct {
	ConfigFilePath string
	Cloud          *Cloud                `json:"cloud,omitempty"`
	Remotes        []Remote              `json:"remotes,omitempty"`
	Boards         []board.Config        `json:"boards,omitempty"`
	Components     []Component           `json:"components,omitempty"`
	Processes      []rexec.ProcessConfig `json:"processes,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (c Config) Validate(fromCloud bool) error {
	if c.Cloud != nil {
		if err := c.Cloud.Validate("cloud", fromCloud); err != nil {
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
func (c Config) FindComponent(name string) *Component {
	for _, cmp := range c.Components {
		if cmp.Name == name {
			return &cmp
		}
	}
	return nil
}

// A Remote describes a remote robot that should be integrated.
type Remote struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Prefix  bool   `json:"prefix"`
}

// Validate ensures all parts of the config are valid.
func (config *Remote) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	if config.Address == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "address")
	}
	return nil
}

// A Cloud describes how to configure a robot controlled by the
// cloud.
// The cloud source could be anything that supports http.
// URL is constructed as $Path?id=ID and secret is put in a http header.
type Cloud struct {
	ID               string        `json:"id"`
	Secret           string        `json:"secret"`
	Self             string        `json:"self"`
	SignalingAddress string        `json:"signaling_address"`
	Path             string        `json:"path,omitempty"`    // optional, defaults to viam cloud otherwise
	LogPath          string        `json:"logPath,omitempty"` // optional, defaults to viam cloud otherwise
	RefreshInterval  time.Duration `json:"refresh_interval,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *Cloud) Validate(path string, fromCloud bool) error {
	if config.ID == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "id")
	}
	if fromCloud {
		if config.Self == "" {
			return utils.NewConfigValidationFieldRequiredError(path, "self")
		}
	} else {
		if config.Secret == "" {
			return utils.NewConfigValidationFieldRequiredError(path, "secret")
		}
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 10 * time.Second
	}
	return nil
}
