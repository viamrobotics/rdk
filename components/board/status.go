package board

import (
	"context"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"
)

// CreateStatus constructs a new up to date status from the given board.
// The operation can take time and be expensive, so it can be cancelled by the
// given context.
func CreateStatus(ctx context.Context, b Board) (*pb.Status, error) {
	var status pb.Status

	if names := b.AnalogNames(); len(names) != 0 {
		status.Analogs = make(map[string]*commonpb.AnalogStatus, len(names))
		for _, name := range names {
			x, err := b.AnalogByName(name)
			if err != nil {
				return nil, err
			}
			val, err := x.Read(ctx, nil)
			if err != nil {
				return nil, errors.Wrapf(err, "couldn't read analog (%s)", name)
			}
			status.Analogs[name] = &commonpb.AnalogStatus{Value: int32(val)}
		}
	}

	if names := b.DigitalInterruptNames(); len(names) != 0 {
		status.DigitalInterrupts = make(map[string]*commonpb.DigitalInterruptStatus, len(names))
		for _, name := range names {
			x, err := b.DigitalInterruptByName(name)
			if err != nil {
				return nil, err
			}
			intVal, err := x.Value(ctx, nil)
			if err != nil {
				return nil, err
			}
			status.DigitalInterrupts[name] = &commonpb.DigitalInterruptStatus{Value: intVal}
		}
	}

	return &status, nil
}
