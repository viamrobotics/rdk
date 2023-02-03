package genericlinux

import (
	"fmt"

	"github.com/d2r2/go-i2c"
)

type I2cBus struct {
	number int
	name   string
}

func newI2cBus(name string, number int) I2cBus {
	return I2cBus{name: name, number: number}
}

// This lets the I2cBus type implement the board.I2C interface.
func (bus *I2cBus) OpenHandle(addr byte) (I2CHandle, error) {
	return i2c.NewI2C(addr, bus.number)
}

// This helps the i2c.I2C struct implement the board.I2CHandle interface.
func (h *i2c.I2C) Write(ctx context.Context, tx []byte) error {
	bytesWritten, err := h.WriteBytes(tx)
	if err != nil {
		return err
	}
	if bytesWritten != len(tx) {
		return fmt.Errorf("Not all bytes were written to I2C address %d on bus %d! Had %d, wrote %d.",
			h.GetAddr(), h.GetBus(), len(tx), bytesWritten)
	}
	return nil
}

// This helps the i2c.I2C struct implement the board.I2CHandle interface.
func (h *i2c.I2C) Read(ctx context.Context, count int) ([]byte, error) {
	buffer := make([]byte, count)
	bytesRead, err := h.ReadBytes(buffer)
	if err != nil {
		return nil, err
	}
	if bytesRead != count {
		return nil, fmt.Errorf("Not enough bytes were read from I2C address %d on bus %d! Needed %d, got %d.",
			h.GetAddr(), h.GetBus(), count, bytesRead)
	}
	return buffer, nil
}

// This helps the i2c.I2C struct implement the board.I2CHandle interface.
func (h *i2c.I2C) ReadByteData(ctx context.Context, register byte) (byte, error) {
	return h.ReadRegU8(register)
}

// This helps the i2c.I2C struct implement the board.I2CHandle interface.
func (h *i2c.I2C) WriteByteData(ctx context.Context, register, data byte) error {
	return h.WriteRegU8(register, data)
}

// This helps the i2c.I2C struct implement the board.I2CHandle interface.
func (h *i2c.I2C) ReadWordData(ctx context.Context, register byte) (uint16, error) {
	return h.ReadRegU16BE(register)
}

// This helps the i2c.I2C struct implement the board.I2CHandle interface.
func (h *i2c.I2C) WriteWordData(ctx context.Context, register byte, data uint16) error {
	return h.WriteRegU16BE(register, data)
}

// This helps the i2c.I2C struct implement the board.I2CHandle interface.
func (h *i2c.I2C) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	// The ignored value is the number of bytes we read. It should be identical to len(results),
	// and if it's ever not, we should probably just return the results anyway.
	results, _, err := h.ReadRegBytes(register, numBytes)
	if err != nil {
		return nil, err
	}
	if len(results) != numBytes {
		return nil, fmt.Errorf("Not enough bytes were read from I2C register %d, address %d on bus %d! Needed %d, got %d.",
			register, h.GetAddr(), h.GetBus(), numBytes, len(results))
	}
	return results, nil
}

// This helps the i2c.I2C struct implement the board.I2CHandle interface.
func (h *i2c.I2C) WriteBlockData(ctx context.Context, register byte, numBytes uint8, data []byte) error {
	// The I2C library we're using doesn't have a specialized "write many bytes to a register"
	// function, but on devices that use registers, this sholud be equivalent to writing the
	// register address and then the relevant bytes.
	rawData := make([]byte, numBytes+1)
	rawData[0] = register
	rawData[1:] = data
	bytesWritten, err := h.WriteBytes(rawData)
	if err != nil {
		return err
	}
	if bytesWritten != numBytes+1 {
		return fmt.Errorf("Not enough bytes were written to I2C register %d, address %d on bus %d! Needed %d, got %d.",
			register, h.GetAddr(), h.GetBus(), numBytes, bytesWritten-1)
	}
	return nil
}
