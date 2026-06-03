// Package status contains types describing the runtime status of a module
// as tracked by the module manager. It is a leaf package so that both the module
// manager and the robot package can depend on it without an import cycle.
package modulestatus

import "time"

// State captures the lifecycle state of a module.
type State uint8

// These states duplicate the ModuleState protos and are documented in the protos.
const (
	ModuleStateUnknown State = iota
	ModuleStatePending
	ModuleStateFirstRun
	ModuleStateStarting
	ModuleStateReady
	ModuleStateUnhealthy
	ModuleStateRemoving
)

// Status encapsulates the current status of a module.
type Status struct {
	Name                string
	State               State
	LastUpdated         time.Time
	Error               error
	ConsecutiveFailures int
}
