// Package control package for feedback loop controls
package control

import (
	"context"
)

// Controllable controllable type for a DC motor.
type Controllable interface {
	// SetPower set the power and direction of the motor
	SetPower(ctx context.Context, power float64, extra map[string]interface{}) error
	// GetPosition returns the current encoder count value
	GetPosition(ctx context.Context, extra map[string]interface{}) (float64, error)
}

// ControlConfig configuration of the control loop
// nolint: revive
type ControlConfig struct {
	Blocks    []ControlBlockConfig `json:"blocks"`    // Blocks Control Block Config
	Frequency float64              `json:"frequency"` // Frequency loop Frequency
}

// Control control interface can be used to interfact with a control loop to query signals, change config, start/stop the loop etc...
type Control interface {
	// OutputAt returns the Signal at the block name, error when the block doesn't exist
	OutputAt(ctx context.Context, name string) ([]Signal, error)
	// ConfigAt returns the Configl at the block name, error when the block doesn't exist
	ConfigAt(ctx context.Context, name string) (ControlBlockConfig, error)
	// BlockList returns the list of blocks in a control loop error when the list is empty
	BlockList(ctx context.Context) ([]string, error)
	// Frequency returns the loop's frequency
	Frequency(ctx context.Context) (float64, error)
	// Start starts the loop
	Start() error
	// Stop stops the loop
	Stop()
}
