// Package state provides apis for motion builtin plan executions
// and manages the state of those executions
package state

import (
	"context"

	"go.viam.com/rdk/logging"
)

// State is the state of the builtin motion service
// It keeps track of the builtin motion service's executions.
type State struct{}

// NewState creates a new state.
func NewState(ctx context.Context, logger logging.Logger) *State {
	s := State{}
	return &s
}

// Stop stops all executions within the State.
func (s *State) Stop() {
}
