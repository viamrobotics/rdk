// Package board defines the interfaces that typically live on a single-board computer
// such as a Raspberry Pi.
//
// Besides the board itself, some other interfaces it defines are motors, servos,
// analog readers, and digital interrupts.
package board

import (
	"context"

	pb "go.viam.com/core/proto/api/v1"
)

// A Board represents a physical general purpose board that contains various
// components such as motors, servos, analog readers, and digital interrupts.
type Board interface {
	// MotorByName returns a motor by name.
	MotorByName(name string) (Motor, bool)

	// ServoByName returns a servo by name.
	ServoByName(name string) (Servo, bool)

	// SPIByName returns an SPI bus by name.
	SPIByName(name string) (SPI, bool)

	// AnalogReaderByName returns an analog reader by name.
	AnalogReaderByName(name string) (AnalogReader, bool)

	// DigitalInterruptByName returns a digital interrupt by name.
	DigitalInterruptByName(name string) (DigitalInterrupt, bool)

	// MotorNames returns the name of all known motors.
	MotorNames() []string

	// ServoNames returns the name of all known servos.
	ServoNames() []string

	// SPINames returns the name of all known SPI busses.
	SPINames() []string

	// AnalogReaderNames returns the name of all known analog readers.
	AnalogReaderNames() []string

	// DigitalInterruptNames returns the name of all known digital interrupts.
	DigitalInterruptNames() []string

	// GPIOSet sets the given pin to either low or high.
	GPIOSet(pin string, high bool) error

	// GPIOGet gets the high/low state of the given pin.
	GPIOGet(pin string) (bool, error)

	// PWMSet sets the given pin to the given duty cycle.
	PWMSet(pin string, dutyCycle byte) error

	// PWMSetFreq sets the given pin to the given PWM frequency. 0 will use the board's default PWM frequency.
	PWMSetFreq(pin string, freq uint) error

	// Status returns the current status of the board. Usually you
	// should use the CreateStatus helper instead of directly calling
	// this.
	Status(ctx context.Context) (*pb.BoardStatus, error)

	// ModelAttributes returns attributes related to the model of this board.
	ModelAttributes() ModelAttributes

	// Close shuts the board down, no methods should be called on the board after this
	Close() error
}

// ModelAttributes provide info related to a board model.
type ModelAttributes struct {
	// Remote signifies this board is accessed over a remote connection.
	// e.g. gRPC
	Remote bool
}

// A Motor represents a physical motor connected to a board.
type Motor interface {

	// Power sets the percentage of power the motor should employ between 0-1.
	Power(ctx context.Context, powerPct float32) error

	// Go instructs the motor to go in a specific direction at a percentage
	// of power between 0-1.
	Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error

	// GoFor instructs the motor to go in a specific direction for a specific amount of
	// revolutions at a given speed in revolutions per minute.
	GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error

	// Position reports the position of the motor based on its encoder. If it's not supported, the returned
	// data is undefined. The unit returned is the number of revolutions which is intended to be fed
	// back into calls of GoFor.
	Position(ctx context.Context) (float64, error)

	// PositionSupported returns whether or not the motor supports reporting of its position which
	// is reliant on having an encoder.
	PositionSupported(ctx context.Context) (bool, error)

	// Off turns the motor off.
	Off(ctx context.Context) error

	// IsOn returns whether or not the motor is currently on.
	IsOn(ctx context.Context) (bool, error)
}

// A Servo represents a physical servo connected to a board.
type Servo interface {

	// Move moves the servo to the given angle (0-180 degrees)
	Move(ctx context.Context, angleDegs uint8) error

	// Current returns the current set angle (degrees) of the servo.
	Current(ctx context.Context) (uint8, error)
}

// SPI represents a shareable SPI bus on the board
// Open() locks the shared bus and returns a handle interface that MUST be closed when done.
type SPI interface {
	Open() (SPIHandle, error)
}

// SPIHandle is similar to an io handle. It MUST be closed.
type SPIHandle interface {
	Xfer(baud uint, chipSelect string, mode uint, tx []byte) ([]byte, error)
	Close() error
}

// An AnalogReader represents an analog pin reader that resides on a board.
type AnalogReader interface {
	// Read reads off the current value.
	Read(ctx context.Context) (int, error)
}

// A PostProcessor takes a raw input and transforms it into a new value.
// Multiple post processors can be stacked on each other. This is currently
// only used in DigitalInterrupt readings.
type PostProcessor func(raw int64) int64

// FlipDirection flips over a relative direction. For example, forward
// flips to backward.
func FlipDirection(d pb.DirectionRelative) pb.DirectionRelative {
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		return pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		return pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	}

	return d
}
