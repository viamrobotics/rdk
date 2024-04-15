package board

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/board/v1"
)

// CreateStatus constructs a new up to date status from the given board.
// The operation can take time and be expensive, so it can be cancelled by the
// given context.
func CreateStatus(ctx context.Context, b Board) (*pb.Status, error) {
	var status pb.Status

	if names := b.AnalogReaderNames(); len(names) != 0 {
		status.Analogs = make(map[string]int32, len(names))
		for _, name := range names {
			x, ok := b.AnalogReaderByName(name)
			if !ok {
				return nil, fmt.Errorf("analog %q not found", name)
			}
			val, err := x.Read(ctx, nil)
			if err != nil {
				return nil, errors.Wrapf(err, "couldn't read analog (%s)", name)
			}
			status.Analogs[name] = int32(val)
		}
	}

	if names := b.DigitalInterruptNames(); len(names) != 0 {
		status.DigitalInterrupts = make(map[string]int64, len(names))
		for _, name := range names {
			x, ok := b.DigitalInterruptByName(name)
			if !ok {
				return nil, fmt.Errorf("digital interrupt %q not found", name)
			}
			intVal, err := x.Value(ctx, nil)
			if err != nil {
				return nil, err
			}
			status.DigitalInterrupts[name] = intVal
		}
	}

	return &status, nil
}
