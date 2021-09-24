// Package board defines the interfaces that typically live on a single-board computer
// such as a Raspberry Pi.
//
// Besides the board itself, some other interfaces it defines are analog readers and digital interrupts.
package board

import (
	"context"

	pb "go.viam.com/core/proto/api/v1"
)

// A Board represents a physical general purpose board that contains various
// components such as analog readers, and digital interrupts.
type Board interface {
	// SPIByName returns an SPI bus by name.
	SPIByName(name string) (SPI, bool)

	// I2CByName returns an I2C bus by name.
	I2CByName(name string) (I2C, bool)

	// AnalogReaderByName returns an analog reader by name.
	AnalogReaderByName(name string) (AnalogReader, bool)

	// DigitalInterruptByName returns a digital interrupt by name.
	DigitalInterruptByName(name string) (DigitalInterrupt, bool)

	// SPINames returns the name of all known SPI busses.
	SPINames() []string

	// I2CNames returns the name of all known I2C busses.
	I2CNames() []string

	// AnalogReaderNames returns the name of all known analog readers.
	AnalogReaderNames() []string

	// DigitalInterruptNames returns the name of all known digital interrupts.
	DigitalInterruptNames() []string

	// GPIOSet sets the given pin to either low or high.
	GPIOSet(ctx context.Context, pin string, high bool) error

	// GPIOGet gets the high/low state of the given pin.
	GPIOGet(ctx context.Context, pin string) (bool, error)

	// PWMSet sets the given pin to the given duty cycle.
	PWMSet(ctx context.Context, pin string, dutyCycle byte) error

	// PWMSetFreq sets the given pin to the given PWM frequency. 0 will use the board's default PWM frequency.
	PWMSetFreq(ctx context.Context, pin string, freq uint) error

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

// SPI represents a shareable SPI bus on the board.
type SPI interface {
	// OpenHandle locks the shared bus and returns a handle interface that MUST be closed when done.
	OpenHandle() (SPIHandle, error)
}

// SPIHandle is similar to an io handle. It MUST be closed to release the bus.
type SPIHandle interface {
	// Xfer performs a single SPI transfer, that is, the complete transaction from chipselect enable to chipselect disable.
	// SPI transfers are synchronous, number of bytes received will be equal to the number of bytes sent.
	// Write-only transfers can usually just discard the returned bytes.
	// Read-only transfers usually transmit a request/address and continue with some number of null bytes to equal the expected size of the returning data.
	// Large transmissions are usually broken up into multiple transfers.
	// There are many different paradigms for most of the above, and implementation details are chip/device specific.
	Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) ([]byte, error)
	// Close closes the handle and releases the lock on the bus.
	Close() error
}

// I2C represents a shareable I2C bus on the board.
type I2C interface {
	// OpenHandle locks the shared bus and returns a handle interface that MUST be closed when done.
	OpenHandle() (I2CHandle, error)
}

// I2CHandle is similar to an io handle. It MUST be closed to release the bus.
type I2CHandle interface {
	Write(ctx context.Context, addr byte, tx []byte) error

	Read(ctx context.Context, addr byte, count int) ([]byte, error)

	// Close closes the handle and releases the lock on the bus.
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
