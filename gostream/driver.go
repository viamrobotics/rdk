package gostream

import (
	"fmt"
	"sync"

	"go.viam.com/utils"
)

// DriverInUseError is returned when closing drivers that still being read from.
type DriverInUseError struct {
	label string
}

func (err *DriverInUseError) Error() string {
	return fmt.Sprintf("driver is still in use: %s", err.label)
}

// driverRefManager is a lockable map of drivers and reference counts of video readers
// that use them.
type driverRefManager struct {
	refs map[string]utils.RefCountedValue
	mu   sync.Mutex
}

// initialize a global driverRefManager.
var driverRefs = driverRefManager{refs: map[string]utils.RefCountedValue{}}
