package board

import (
	"context"

	pb "go.viam.com/robotcore/proto/api/v1"
)

type PostProcess func(raw int64) int64

func FlipDirection(d pb.DirectionRelative) pb.DirectionRelative {
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		return pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		return pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	}

	return d
}

type Motor interface {
	// Power sets the percentage of power the motor should employ between 0-1.
	Power(ctx context.Context, powerPct float32) error

	// Go instructs the motor to go in a specific direction at a percentage
	// of power between 0-1.
	Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error

	GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error

	// this is only supported if you have an encocder, return will be garbage if PositionSupported is false
	// the unit is revolutions so that it can be used for relative GoFor commands
	Position(ctx context.Context) (float64, error)
	PositionSupported(ctx context.Context) (bool, error)

	Off(ctx context.Context) error
	IsOn(ctx context.Context) (bool, error)
}

type Servo interface {
	// moves to that angle (0-180)
	Move(ctx context.Context, angle uint8) error
	Current(ctx context.Context) (uint8, error)
}

type AnalogReader interface {
	Read(ctx context.Context) (int, error)
}

type Board interface {
	// nil if cannot find
	Motor(name string) Motor
	Servo(name string) Servo

	AnalogReader(name string) AnalogReader
	DigitalInterrupt(name string) DigitalInterrupt

	GetConfig(ctx context.Context) (Config, error)

	// should use CreateStatus in most cases
	Status(ctx context.Context) (*pb.BoardStatus, error)
}

type DigitalInterrupt interface {
	Config() DigitalInterruptConfig
	Value() int64

	// nanos is from an arbitrary point in time, but always increasing and always needs to be accurate
	// using time.Now().UnixNano() would be acceptable, but not required
	Tick(high bool, nanos uint64)
	AddCallback(c chan bool)
	AddPostProcess(pp PostProcess)
}
