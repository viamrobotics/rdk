//go:build !linux

package genericlinux

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
)

// Trying to run a genericlinux board on a non-Linux OS!? That won't work. We'll provide the same
// public interface, but it won't do anything. We don't even log warnings because this gets called
// to add the boards to the registry, and we just need to ensure that they never get used (by never
// actually adding them to the registry).
func RegisterBoard(modelName string, gpioMappings map[int]GPIOBoardMapping, usePeriphGpio bool) {
}
