// Package buses offers SPI and I2C buses for generic Linux systems.
package buses

import (
	"context"
)

// I2C represents a shareable I2C bus on the board.
type I2C interface {
	// OpenHandle locks returns a handle interface that MUST be closed when done.
	// you cannot have 2 open for the same addr
	OpenHandle(addr byte) (I2CHandle, error)
}

// I2CHandle is similar to an io handle. It MUST be closed to release the bus.
type I2CHandle interface {
	Write(ctx context.Context, tx []byte) error
	Read(ctx context.Context, count int) ([]byte, error)

	ReadByteData(ctx context.Context, register byte) (byte, error)
	WriteByteData(ctx context.Context, register, data byte) error

	ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error)
	WriteBlockData(ctx context.Context, register byte, data []byte) error

	// Close closes the handle and releases the lock on the bus.
	Close() error
}

// An I2CRegister is a lightweight wrapper around a handle for a particular register.
type I2CRegister struct {
	Handle   I2CHandle
	Register byte
}

// ReadByteData reads a byte from the I2C channel register.
func (reg *I2CRegister) ReadByteData(ctx context.Context) (byte, error) {
	return reg.Handle.ReadByteData(ctx, reg.Register)
}

// WriteByteData writes a byte to the I2C channel register.
func (reg *I2CRegister) WriteByteData(ctx context.Context, data byte) error {
	return reg.Handle.WriteByteData(ctx, reg.Register, data)
}
