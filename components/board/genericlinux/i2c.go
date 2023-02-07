package genericlinux

import (
	"binary"
	"context"
	"fmt"
	"sync"

	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/host"

	"go.viam.com/rdk/components/board"
)

func init() {
	host.Init()
}

type i2cBus struct {
	internal i2c.BusCloser
	mu       sync.Mutex
}

func NewI2cBus(deviceName string) (i2cBus, error) {
	bus, err := i2creg.Open(deviceName)
	if err != nil {
		return nil, err
	}
	return i2cBus{internal: bus}

}

// This lets the i2cBus type implement the board.I2C interface.
func (bus *i2cBus) OpenHandle(addr byte) (board.I2CHandle, error) {
	bus.mu.Lock() // Lock the bus so no other handle can use it until this one is closed.
	return &i2cHandle{device: i2c.Dev{Bus: bus.internal, Addr: addr}, mu: &bus.mu}, nil
}

type i2cHandle struct {
	device   i2c.Dev
	mu       *sync.Mutex // Points to the I2C bus' mutex
}

// This helps the i2cHandle struct implement the board.I2CHandle interface.
func (h *i2cHandle) Write(ctx context.Context, tx []byte) error {
	return h.device.Tx(tx, nil)
}

// This helps the i2cHandle struct implement the board.I2CHandle interface.
func (h *i2cHandle) Read(ctx context.Context, count int) ([]byte, error) {
	buffer := make([]byte, count)
	err := h.device.Tx(nil, buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

// This helps the i2cHandle struct implement the board.I2CHandle interface.
func (h *i2cHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	buffer, err := h.Read(ctx, 1)
	if err != nil {
		return 0, err
	}
	return buffer[0], nil
}

// This is a private helper function, used to implement the rest of the board.I2CHandle interface.
func (h *i2cHandle) transactAtRegister(register byte, w, r []byte) error {
	if w == nil {
		w := []byte{}
	}
	fullW := make([]byte, len(w) + 1)
	fullW[0] = register
	copy(fullW[1:], w)
	return h.device.Tx(fullW, r)
}

// This helps the i2cHandle struct implement the board.I2CHandle interface.
func (h *i2cHandle) WriteByteData(ctx context.Context, register, data byte) error {
	return h.transactAtRegister(register, []byte{data}, nil)
}

// This helps the i2cHandle struct implement the board.I2CHandle interface.
func (h *i2cHandle) ReadWordData(ctx context.Context, register byte) (uint16, error) {
	result := make([]byte, 2)
	err := h.transactAtRegister(register, nil, result)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(result), nil
}

// This helps the i2cHandle struct implement the board.I2CHandle interface.
func (h *i2cHandle) WriteWordData(ctx context.Context, register byte, data uint16) error {
	w := make([]byte, 2)
	binary.BigEndian.PutUint16(w[:], data)
	return h.transactAtRegister(register, w, nil)
}

// This helps the i2cHandle struct implement the board.I2CHandle interface.
func (h *i2cHandle) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	result := make([]byte, numBytes)
	err := h.transactAtRegister(register, nil, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// This helps the i2cHandle struct implement the board.I2CHandle interface.
func (h *i2cHandle) WriteBlockData(ctx context.Context, register byte, numBytes uint8, data []byte) error {
	return h.transactAtRegister(register, data, nil)
}

func (h *i2cHandle) Close() error {
	defer h.mu.Unlock() // Unlock the entire bus so someone else can use it
	h.device = nil
	// Don't close the bus itself: it should remain open for other handles to use
}
