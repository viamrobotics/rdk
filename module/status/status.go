// Package status contains types describing the runtime status of a module
// as tracked by the module manager. It is a leaf package so that both the module
// manager and the robot package can depend on it without an import cycle.
package status

import "time"

// State captures the lifecycle state of a module.
type State uint8

// These states duplicate the ModuleState protos and are documented in the protos.
const (
	ModuleStateUnknown State = iota
	ModuleStatePending
	ModuleStateStarting
	ModuleStateReady
	ModuleStateUnhealthy
	ModuleStateClosing
)

// Status represents the status of a module tracked by the module manager
type Status struct {
	Name                string
	State               State
	LastUpdated         time.Time
	Error               error
	ConsecutiveFailures uint
	// Version is the package version the module most recently reached ready with. It is not
	// cleared when the module leaves ready, so check State before treating it as running.
	Version string
}
