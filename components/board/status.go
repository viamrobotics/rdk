package board

import (
	"context"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/board/v1"
)

// CreateStatus constructs a new up to date status from the given board.
// The operation can take time and be expensive, so it can be cancelled by the
// given context.
func CreateStatus(ctx context.Context, b Board) (*pb.Status, error) {
	var status pb.Status

	if names := b.AnalogNames(); len(names) != 0 {
		status.Analogs = make(map[string]int32, len(names))
		for _, name := range names {
			x, err := b.AnalogByName(name)
			if err != nil {
				return nil, err
			}
			val, err := x.Read(ctx, nil)
			if err != nil {
				return nil, errors.Wrapf(err, "couldn't read analog (%s)", name)
			}
			status.Analogs[name] = int32(val.Value)
		}
	}

	if names := b.DigitalInterruptNames(); len(names) != 0 {
		status.DigitalInterrupts = make(map[string]int64, len(names))
		for _, name := range names {
			x, err := b.DigitalInterruptByName(name)
			if err != nil {
				return nil, err
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
