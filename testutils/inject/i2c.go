package inject

import (
	"context"

	"go.viam.com/rdk/components/board/genericlinux/buses"
)

// I2C is an injected I2C.
type I2C struct {
	buses.I2C
	OpenHandleFunc func(addr byte) (buses.I2CHandle, error)
}

// OpenHandle calls the injected OpenHandle or the real version.
func (s *I2C) OpenHandle(addr byte) (buses.I2CHandle, error) {
	if s.OpenHandleFunc == nil {
		return s.I2C.OpenHandle(addr)
	}
	return s.OpenHandleFunc(addr)
}

// I2CHandle is an injected I2CHandle.
type I2CHandle struct {
	buses.I2CHandle
	WriteFunc          func(ctx context.Context, tx []byte) error
	ReadFunc           func(ctx context.Context, count int) ([]byte, error)
	ReadByteDataFunc   func(ctx context.Context, register byte) (byte, error)
	WriteByteDataFunc  func(ctx context.Context, register, data byte) error
	ReadBlockDataFunc  func(ctx context.Context, register byte, numBytes uint8) ([]byte, error)
	WriteBlockDataFunc func(ctx context.Context, register byte, data []byte) error
	CloseFunc          func() error
}

// ReadByteData calls the injected ReadByteDataFunc or the real version.
func (handle *I2CHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	if handle.ReadByteDataFunc == nil {
		return handle.I2CHandle.ReadByteData(ctx, register)
	}
	return handle.ReadByteDataFunc(ctx, register)
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

// WriteBlockData calls the injected WriteBlockDataFunc or the real version.
func (handle *I2CHandle) WriteBlockData(ctx context.Context, register byte, data []byte) error {
	if handle.WriteBlockDataFunc == nil {
		return handle.I2CHandle.WriteBlockData(ctx, register, data)
	}
	return handle.WriteBlockDataFunc(ctx, register, data)
}

// Read calls the injected ReadFunc or the real version.
func (handle *I2CHandle) Read(ctx context.Context, count int) ([]byte, error) {
	if handle.ReadFunc == nil {
		return handle.I2CHandle.Read(ctx, count)
	}
	return handle.ReadFunc(ctx, count)
}

// Write calls the injected WriteFunc or the real version.
func (handle *I2CHandle) Write(ctx context.Context, tx []byte) error {
	if handle.WriteFunc == nil {
		return handle.I2CHandle.Write(ctx, tx)
	}
	return handle.WriteFunc(ctx, tx)
}

// Close calls the injected CloseFunc or the real version.
func (handle *I2CHandle) Close() error {
	if handle.CloseFunc == nil {
		return handle.I2CHandle.Close()
	}
	return handle.CloseFunc()
}
