package inject

import (
	"context"

	"go.viam.com/rdk/components/board"
)

// SPI is an injected SPI.
type SPI struct {
	board.SPI
	OpenHandleFunc func() (board.SPIHandle, error)
}

// OpenHandle calls the injected OpenHandle or the real version.
func (s *SPI) OpenHandle() (board.SPIHandle, error) {
	if s.OpenHandleFunc == nil {
		return s.SPI.OpenHandle()
	}
	return s.OpenHandleFunc()
}

// SPIHandle is an injected connection to an SPI bus.
type SPIHandle struct {
	board.SPIHandle
	XferFunc  func(context.Context, uint, string, uint, []byte) ([]byte, error)
	CloseFunc func() error
}

// Xfer calls the injected XferFunc or the real version.
func (s *SPIHandle) Xfer(
	ctx context.Context,
	baud uint,
	chipSelect string,
	mode uint,
	tx []byte,
) ([]byte, error) {
	if s.XferFunc == nil {
		return s.XferFunc(ctx, baud, chipSelect, mode, tx)
	}
	return s.Xfer(ctx, baud, chipSelect, mode, tx)
}

// Close calls the injected CloseFunc or the real version.
func (s *SPIHandle) Close() error {
	if s.CloseFunc == nil {
		return s.CloseFunc()
	}
	return s.Close()
}
