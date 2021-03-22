package board

type PostProcess func(raw int64) int64

type Direction int

const (
	DirNone     = Direction(0)
	DirForward  = Direction(1)
	DirBackward = Direction(2)
)

func DirectionFromString(s string) Direction {
	if len(s) == 0 {
		return DirNone
	}

	if s[0] == 'f' {
		return DirForward
	}

	if s[0] == 'b' {
		return DirBackward
	}

	return DirNone
}

type Motor interface {
	Force(force byte) error

	Go(d Direction, force byte) error

	GoFor(d Direction, rpm float64, rotations float64) error

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
	Status() (Status, error)
}

type DigitalInterrupt interface {
	Config() DigitalInterruptConfig
	Value() int64
	Tick(high bool)
	AddCallback(c chan bool)
	AddPostProcess(pp PostProcess)
}
