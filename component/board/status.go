package board

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	pb "go.viam.com/core/proto/api/v1"
)

// CreateStatus constructs a new up to date status from the given board.
// The operation can take time and be expensive, so it can be cancelled by the
// given context.
func CreateStatus(ctx context.Context, b Board) (*pb.BoardStatus, error) {
	var status pb.BoardStatus

	if names := b.AnalogReaderNames(); len(names) != 0 {
		status.Analogs = make(map[string]*pb.AnalogStatus, len(names))
		for _, name := range names {
			x, ok := b.AnalogReaderByName(name)
			if !ok {
				return nil, fmt.Errorf("analog %q not found", name)
			}
			val, err := x.Read(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "couldn't read analog (%s)")
			}
			status.Analogs[name] = &pb.AnalogStatus{Value: int32(val)}
		}
	}

	if names := b.DigitalInterruptNames(); len(names) != 0 {
		status.DigitalInterrupts = make(map[string]*pb.DigitalInterruptStatus, len(names))
		for _, name := range names {
			x, ok := b.DigitalInterruptByName(name)
			if !ok {
				return nil, fmt.Errorf("digital interrupt %q not found", name)
			}
			intVal, err := x.Value(ctx)
			if err != nil {
				return nil, err
			}
			status.DigitalInterrupts[name] = &pb.DigitalInterruptStatus{Value: intVal}
		}
	}

	return &status, nil
}
