package board

import (
	"context"

	"github.com/edaniels/golog"
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

// ReadByteDataFromBus opens a handle for the bus adhoc to perform a single read of one byte
// of data and returns the result. The handle is closed at the end.
func ReadByteDataFromBus(ctx context.Context, bus I2C, addr, register byte) (byte, error) {
	i2cHandle, err := bus.OpenHandle(addr)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := i2cHandle.Close(); err != nil {
			golog.Global().Error(err)
		}
	}()
	return i2cHandle.ReadByteData(ctx, register)
}

// WriteByteDataToBus opens a handle for the bus adhoc to perform a single write of
// one byte of data. The handle is closed at the end.
func WriteByteDataToBus(ctx context.Context, bus I2C, addr, register, data byte) error {
	i2cHandle, err := bus.OpenHandle(addr)
	if err != nil {
		return err
	}
	defer func() {
		if err := i2cHandle.Close(); err != nil {
			golog.Global().Error(err)
		}
	}()
	return i2cHandle.WriteByteData(ctx, register, data)
}

// WriteToBus opens a handle for the bus adhoc to perform a single write.
// The handle is closed at the end.
func WriteToBus(ctx context.Context, bus I2C, addr byte, tx []byte) error {
	i2cHandle, err := bus.OpenHandle(addr)
	if err != nil {
		return err
	}
	defer func() {
		if err := i2cHandle.Close(); err != nil {
			golog.Global().Error(err)
		}
	}()
	return i2cHandle.Write(ctx, tx)
}
