//go:build linux

// Package customlinux implements a board running Linux
package customlinux

import (
	"os"
)

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	BoardDefsFilePath string            `json:"board_defs_file_path"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if _, err := os.Stat(conf.BoardDefsFilePath); err != nil {
		return nil, err
	}
	// Should we read in and validate the board defs in here?

	return nil, nil
}
