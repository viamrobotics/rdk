package board

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
)

// CreateStatus constructs a new up to date status from the given board.
// The operation can take time and be expensive, so it can be cancelled by the
// given context.
func CreateStatus(ctx context.Context, b Board) (*commonpb.BoardStatus, error) {
	var status commonpb.BoardStatus

	if names := b.AnalogReaderNames(); len(names) != 0 {
		status.Analogs = make(map[string]*commonpb.AnalogStatus, len(names))
		for _, name := range names {
			x, ok := b.AnalogReaderByName(name)
			if !ok {
				return nil, fmt.Errorf("analog %q not found", name)
			}
			val, err := x.Read(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "couldn't read analog (%s)")
			}
			status.Analogs[name] = &commonpb.AnalogStatus{Value: int32(val)}
		}
	}

	if names := b.DigitalInterruptNames(); len(names) != 0 {
		status.DigitalInterrupts = make(map[string]*commonpb.DigitalInterruptStatus, len(names))
		for _, name := range names {
			x, ok := b.DigitalInterruptByName(name)
			if !ok {
				return nil, fmt.Errorf("digital interrupt %q not found", name)
			}
			intVal, err := x.Value(ctx)
			if err != nil {
				return nil, err
			}
			status.DigitalInterrupts[name] = &commonpb.DigitalInterruptStatus{Value: intVal}
		}
	}

	if names := b.GPIOPinNames(); len(names) != 0 {
		status.GpioPins = make(map[string]*commonpb.GPIOPinStatus, len(names))
		for _, name := range names {
			x, err := b.GPIOPinByName(name)
			if err != nil {
				return nil, fmt.Errorf("GPIO pin %q not found", name)
			}
			high, err := x.Get(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "couldn't get GPIO pin status (%s)")
			}
			status.GpioPins[name] = &commonpb.GPIOPinStatus{High: high}
		}
	}

	return &status, nil
}
