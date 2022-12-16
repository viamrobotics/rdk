package board

import (
	"context"

	"go.viam.com/utils"
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

	ReadWordData(ctx context.Context, register byte) (uint16, error)
	WriteWordData(ctx context.Context, register byte, data uint16) error

	ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error)
	WriteBlockData(ctx context.Context, register byte, numBytes uint8, data []byte) error

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

// ReadWordData reads a word from the I2C channel register.
func (reg *I2CRegister) ReadWordData(ctx context.Context) (uint16, error) {
	return reg.Handle.ReadWordData(ctx, reg.Register)
}

// WriteWordData writes a word to the I2C channel register.
func (reg *I2CRegister) WriteWordData(ctx context.Context, data uint16) error {
	return reg.Handle.WriteWordData(ctx, reg.Register, data)
}

type I2CAttrConfig struct {
	I2CBus      string `json:"i2c_bus"`
	I2cAddr     int    `json:"i2c_addr,omitempty"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`
}

// ValidateI2C ensures all parts of the config are valid.
func (cfg *I2CAttrConfig) ValidateI2C(path string, requireAddress bool) error {
	if cfg.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if (cfg.I2cAddr == 0) && requireAddress {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}

	return nil
}
