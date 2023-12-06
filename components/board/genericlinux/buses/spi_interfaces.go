package buses

import (
	"context"
)

// SPI represents a shareable SPI bus on a generic Linux board.
type SPI interface {
	// OpenHandle locks the shared bus and returns a handle interface that MUST be closed when done.
	OpenHandle() (SPIHandle, error)
	Close(ctx context.Context) error
}

// SPIHandle is similar to an io handle. It MUST be closed to release the bus.
type SPIHandle interface {
	// Xfer performs a single SPI transfer, that is, the complete transaction from chipselect
	// enable to chipselect disable. SPI transfers are synchronous, number of bytes received will
	// be equal to the number of bytes sent. Write-only transfers can usually just discard the
	// returned bytes. Read-only transfers usually transmit a request/address and continue with
	// some number of null bytes to equal the expected size of the returning data. Large
	// transmissions are usually broken up into multiple transfers. There are many different
	// paradigms for most of the above, and implementation details are chip/device specific.
	Xfer(
		ctx context.Context,
		baud uint,
		chipSelect string,
		mode uint,
		tx []byte,
	) ([]byte, error)

	// Close closes the handle and releases the lock on the bus.
	Close() error
}
