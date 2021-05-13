// Package rexec defines process management utilities to be used as a library within
// a go process wishing to own sub-processes.
//
// It helps manage the lifecycle of processes by keeping them up as long as possible
// when configured.
package rexec

import "go.viam.com/core/utils"

// A ProcessConfig describes how to manage a system process.
type ProcessConfig struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Args    []string `json:"args"`
	CWD     string   `json:"cwd"`
	OneShot bool     `json:"one_shot"`
	Log     bool     `json:"log"`
}

// Validate ensures all parts of the config are valid.
func (config *ProcessConfig) Validate(path string) error {
	if config.ID == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "id")
	}
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}
