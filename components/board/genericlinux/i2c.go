//go:build linux

// Package genericlinux is for boards that run Linux. This file is for I2C support on those boards.
package genericlinux

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/components/board"
)

func init() {
	if _, err := host.Init(); err != nil {
		golog.Global().Debugw("error initializing host", "error", err)
	}
}

type i2cBus struct {
	// Despite the type name BusCloser, this is the I2C bus itself (plus a way to close itself when
	// it's done, though we never use that because we want to keep it open until the entire process
	// exits)!
	closeableBus i2c.BusCloser
	mu           sync.Mutex
}

func newI2cBus(deviceName string) (*i2cBus, error) {
	// We return a pointer to an i2cBus instead of an i2cBus itself so that we can return nil if
	// something goes wrong.
	bus, err := i2creg.Open(deviceName)
	if err != nil {
		return nil, err
	}
	return &i2cBus{closeableBus: bus}, nil
}

// This lets the i2cBus type implement the board.I2C interface.
func (bus *i2cBus) OpenHandle(addr byte) (board.I2CHandle, error) {
	bus.mu.Lock() // Lock the bus so no other handle can use it until this one is closed.
	return &i2cHandle{device: &i2c.Dev{Bus: bus.closeableBus, Addr: uint16(addr)}, mu: &bus.mu}, nil
}

type i2cHandle struct { // Implements the board.I2CHandle interface
	device *i2c.Dev    // Will become nil if we Close() the handle
	mu     *sync.Mutex // Points to the i2cBus' mutex
}

func (h *i2cHandle) Write(ctx context.Context, tx []byte) error {
	return h.device.Tx(tx, nil)
}

func (h *i2cHandle) Read(ctx context.Context, count int) ([]byte, error) {
	buffer := make([]byte, count)
	err := h.device.Tx(nil, buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

// This is a private helper function, used to implement the rest of the board.I2CHandle interface.
func (h *i2cHandle) transactAtRegister(register byte, w, r []byte) error {
	if w == nil {
		w = []byte{}
	}
	fullW := make([]byte, len(w)+1)
	fullW[0] = register
	copy(fullW[1:], w)
	return h.device.Tx(fullW, r)
}

func (h *i2cHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	result := make([]byte, 1)
	err := h.transactAtRegister(register, nil, result)
	if err != nil {
		return 0, err
	}
	return result[0], nil
}

func (h *i2cHandle) WriteByteData(ctx context.Context, register, data byte) error {
	return h.transactAtRegister(register, []byte{data}, nil)
}

func (h *i2cHandle) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	result := make([]byte, numBytes)
	err := h.transactAtRegister(register, nil, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *i2cHandle) WriteBlockData(ctx context.Context, register byte, data []byte) error {
	return h.transactAtRegister(register, data, nil)
}

func (h *i2cHandle) Close() error {
	defer h.mu.Unlock() // Unlock the entire bus so someone else can use it
	h.device = nil
	// Don't close the bus itself: it should remain open for other handles to use
	return nil
}
