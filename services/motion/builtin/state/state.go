// Package state provides apis for motion builtin plan executions
// and manages the state of those executions
package state

import (
	"context"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
)

// Waypoints represent the waypoints of the plan.
type Waypoints [][]referenceframe.Input

// PlanResponse is the response from Plan.
type PlanResponse struct {
	Waypoints        Waypoints
	Motionplan       motionplan.Plan
	PosesByComponent []motion.PlanStep
}

// ExecuteResponse is the response from Execute.
type ExecuteResponse struct {
	// If true, the Execute function didn't reach the goal & the caller should replan
	Replan bool
	// Set if Replan is true, describes why replanning was triggered
	ReplanReason string
}

// PlanExecutorConstructor creates a PlannerExecutor
// if ctx is cancelled then all PlannerExecutor interface
// methods must terminate & return errors
// req is the request that will be used during planning & execution
// seedPlan (nil during the first plan) is the previous plan
// if replanning has occurred
// replanCount is the number of times replanning has occurred,
// zero the first time planning occurs.
// R is a genric type which is able to be used to create a PlannerExecutor.
type PlanExecutorConstructor[R any] func(
	ctx context.Context,
	req R,
	seedPlan motionplan.Plan,
	replanCount int,
) (PlannerExecutor, error)

// PlannerExecutor implements Plan and Execute.
type PlannerExecutor interface {
	Plan() (PlanResponse, error)
	Execute(Waypoints) (ExecuteResponse, error)
	Cancel()
}

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
