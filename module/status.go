package module

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
type ModuleStatus struct {
	Name                string
	State               State
	LastUpdated         time.Time
	Error               error
	ConsecutiveFailures uint
}
