package board

import pb "go.viam.com/robotcore/proto/api/v1"

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
	Force(force byte) error

	Go(d pb.DirectionRelative, force byte) error

	GoFor(d pb.DirectionRelative, rpm float64, rotations float64) error

	// this is only supported if you have an encocder, return will be garbage if PositionSupported is false
	Position() int64
	PositionSupported() bool

	Off() error
	IsOn() bool
}

type Servo interface {
	// moves to that angle (0-180)
	Move(angle uint8) error
	Current() uint8
}

type AnalogReader interface {
	Read() (int, error)
}

type Board interface {
	// nil if cannot find
	Motor(name string) Motor
	Servo(name string) Servo

	AnalogReader(name string) AnalogReader
	DigitalInterrupt(name string) DigitalInterrupt

	Close() error

	GetConfig() Config

	// should use CreateStatus in most cases
	Status() (*pb.BoardStatus, error)
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
