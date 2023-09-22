//go:build linux

// Package customlinux implements a board running Linux
package customlinux

import (
	"os"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
)

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	BoardDefsFilePath string                         `json:"board_defs_file_path"`
	I2Cs              []board.I2CConfig              `json:"i2cs,omitempty"`
	SPIs              []board.SPIConfig              `json:"spis,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if _, err := os.Stat(conf.BoardDefsFilePath); err != nil {
		return nil, err
	}

	boardConfig := genericlinux.Config{
        I2Cs: conf.I2Cs,
        SPIs: conf.SPIs,
    }
	if deps, err := boardConfig.Validate(path); err != nil {
		return deps, err
	}
	return nil, nil
}
