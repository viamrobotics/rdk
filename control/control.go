// Package control package for feedback loop controls
// This is an Experimental package
package control

import (
	"context"
)

// Controllable controllable type for a DC motor.
type Controllable interface {
	// SetState set the power and direction of the motor
	SetState(ctx context.Context, state []*Signal) error
	// Position returns the current encoder count value
	State(ctx context.Context) ([]float64, error)
}

// PIDConfig configures PID values for the control loop.
type PIDConfig struct {
	P *float64 `json:"p,omitempty"`
	I *float64 `json:"i,omitempty"`
	D *float64 `json:"d,omitempty"`
}

// Config configuration of the control loop.
type Config struct {
	Blocks    []BlockConfig `json:"blocks"`    // Blocks Control Block Config
	Frequency float64       `json:"frequency"` // Frequency loop Frequency
}

// Control control interface can be used to interfact with a control loop to query signals, change config, start/stop the loop etc...
type Control interface {
	// OutputAt returns the Signal at the block name, error when the block doesn't exist
	OutputAt(ctx context.Context, name string) ([]*Signal, error)
	// ConfigAt returns the Configl at the block name, error when the block doesn't exist
	ConfigAt(ctx context.Context, name string) (BlockConfig, error)
	// BlockList returns the list of blocks in a control loop error when the list is empty
	BlockList(ctx context.Context) ([]string, error)
	// Frequency returns the loop's frequency
	Frequency(ctx context.Context) (float64, error)
	// Start starts the loop
	Start() error
	// Stop stops the loop
	Stop()
}
