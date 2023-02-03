package genericlinux

import (
	"context"
	"fmt"

	"github.com/d2r2/go-i2c"
	"go.viam.com/rdk/components/board"
)

type i2cBus struct {
	number int
	name   string
}

// This lets the i2cBus type implement the board.I2C interface.
func (bus *i2cBus) OpenHandle(addr byte) (board.I2CHandle, error) {
	handle, err := i2c.NewI2C(addr, bus.number)
	if err != nil {
		return nil, err
	}
	return &localI2c{internal: handle}, nil
}

// We want to use the i2c.I2C struct, but we also want to have it conform to the board.I2CHandle
// interface, and we cannot define new functions on non-local types. So, we create a local struct
// that contains the non-local one, upon which we can define extra functions.
type localI2c struct {
	internal i2c.I2C
}

// This helps the localI2c struct implement the board.I2CHandle interface.
func (h *localI2c) Write(ctx context.Context, tx []byte) error {
	bytesWritten, err := h.internal.WriteBytes(tx)
	if err != nil {
		return err
	}
	if int(bytesWritten) != len(tx) {
		return fmt.Errorf("Not all bytes were written to I2C address %d on bus %d! Had %d, wrote %d.",
			h.internal.GetAddr(), h.internal.GetBus(), len(tx), bytesWritten)
	}
	return nil
}

// This helps the localI2c struct implement the board.I2CHandle interface.
func (h *localI2c) Read(ctx context.Context, count int) ([]byte, error) {
	buffer := make([]byte, count)
	bytesRead, err := h.internal.ReadBytes(buffer)
	if err != nil {
		return nil, err
	}
	if int(bytesRead) != count {
		return nil, fmt.Errorf("Not enough bytes were read from I2C address %d on bus %d! Needed %d, got %d.",
			h.internal.GetAddr(), h.internal.GetBus(), count, bytesRead)
	}
	return buffer, nil
}

// This helps the localI2c struct implement the board.I2CHandle interface.
func (h *localI2c) ReadByteData(ctx context.Context, register byte) (byte, error) {
	return h.internal.ReadRegU8(register)
}

// This helps the localI2c struct implement the board.I2CHandle interface.
func (h *localI2c) WriteByteData(ctx context.Context, register, data byte) error {
	return h.internal.WriteRegU8(register, data)
}

// This helps the localI2c struct implement the board.I2CHandle interface.
func (h *localI2c) ReadWordData(ctx context.Context, register byte) (uint16, error) {
	return h.internal.ReadRegU16BE(register)
}

// This helps the localI2c struct implement the board.I2CHandle interface.
func (h *localI2c) WriteWordData(ctx context.Context, register byte, data uint16) error {
	return h.internal.WriteRegU16BE(register, data)
}

// This helps the localI2c struct implement the board.I2CHandle interface.
func (h *localI2c) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	// The ignored value is the number of bytes we read. It should be identical to len(results),
	// and if it's ever not, we should probably just return the results anyway.
	results, _, err := h.internal.ReadRegBytes(register, numBytes)
	if err != nil {
		return nil, err
	}
	if len(results) != int(numBytes) {
		return nil, fmt.Errorf("Not enough bytes were read from I2C register %d, address %d on bus %d! Needed %d, got %d.",
			register, h.internal.GetAddr(), h.internal.GetBus(), numBytes, len(results))
	}
	return results, nil
}

// This helps the localI2c struct implement the board.I2CHandle interface.
func (h *localI2c) WriteBlockData(ctx context.Context, register byte, numBytes uint8, data []byte) error {
	// The I2C library we're using doesn't have a specialized "write many bytes to a register"
	// function, but on devices that use registers, this sholud be equivalent to writing the
	// register address and then the relevant bytes.
	rawData := make([]byte, numBytes+1)
	rawData[0] = register
	rawData[1:] = data
	bytesWritten, err := h.internal.WriteBytes(rawData)
	if err != nil {
		return err
	}
	if int(bytesWritten) != int(numBytes)+1 {
		return fmt.Errorf("Not enough bytes were written to I2C register %d, address %d on bus %d! Needed %d, got %d.",
			register, h.internal.GetAddr(), h.internal.GetBus(), numBytes, bytesWritten-1)
	}
	return nil
}

func (h *localI2c) Close() error {
	return h.internal.Close()
}
