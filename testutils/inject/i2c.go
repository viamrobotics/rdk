package inject

import (
	"context"

	"go.viam.com/rdk/components/board"
)

// I2C is an injected I2C.
type I2C struct {
	board.I2C
	OpenHandleFunc func(addr byte) (board.I2CHandle, error)
}

// OpenHandle calls the injected OpenHandle or the real version.
func (s *I2C) OpenHandle(addr byte) (board.I2CHandle, error) {
	if s.OpenHandleFunc == nil {
		return s.I2C.OpenHandle(addr)
	}
	return s.OpenHandleFunc(addr)
}

// I2CHandle is an injected I2CHandle.
type I2CHandle struct {
	board.I2CHandle
	WriteFunc          func(context.Context, []byte) error
	ReadFunc           func(context.Context, int) ([]byte, error)
	ReadByteDataFunc   func(context.Context, byte) (byte, error)
	WriteByteDataFunc  func(context.Context, byte, byte) error
	ReadBlockDataFunc  func(context.Context, byte, uint8) ([]byte, error)
	WriteBlockDataFunc func(context.Context, byte, []byte) error
	CloseFunc          func() error
}

// WriteByteData calls the injected WriteByteDataFunc or the real version.
func (handle *I2CHandle) WriteByteData(ctx context.Context, register, data byte) error {
	if handle.WriteByteDataFunc == nil {
		return handle.I2CHandle.WriteByteData(ctx, register, data)
	}
	return handle.WriteByteDataFunc(ctx, register, data)
}

// ReadBlockData calls the injected ReadBlockDataFunc or the real version.
func (handle *I2CHandle) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	if handle.ReadBlockDataFunc == nil {
		return handle.I2CHandle.ReadBlockData(ctx, register, numBytes)
	}
	return handle.ReadBlockDataFunc(ctx, register, numBytes)
}

// Close calls the injected CloseFunc or the real version.
func (handle *I2CHandle) Close() error {
	if handle.CloseFunc == nil {
		return handle.I2CHandle.Close()
	}
	return handle.CloseFunc()
}
