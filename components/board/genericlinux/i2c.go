//go:build linux

// Package genericlinux is for boards that run Linux. This file is for I2C support on those boards.
package genericlinux

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/resource"
)

func init() {
	if _, err := host.Init(); err != nil {
		golog.Global().Debugw("error initializing host", "error", err)
	}
}

// I2cBus represents an I2C bus. You can use it to create handles for devices at specific
// addresses on the bus. Creating a handle locks the bus, and closing the handle unlocks the bus
// again, so that you can only communicate with 1 device on the bus at a time.
type I2cBus struct {
	// Despite the type name BusCloser, this is the I2C bus itself (plus a way to close itself when
	// it's done, though we never use that because we want to keep it open until the entire process
	// exits)!
	closeableBus i2c.BusCloser
	mu           sync.Mutex
	deviceName   string
}

// NewI2cBus creates a new I2cBus object.
func NewI2cBus(deviceName string) (*I2cBus, error) {
	// We return a pointer to an I2cBus instead of an I2cBus itself so that we can return nil if
	// something goes wrong.
	b := &I2cBus{}
	if err := b.reset(deviceName); err != nil {
		return nil, err
	}
	return b, nil
}

func (bus *I2cBus) reset(deviceName string) error {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	if bus.closeableBus != nil { // Close any old bus we used to have
		if err := bus.closeableBus.Close(); err != nil {
			return err
		}
		bus.closeableBus = nil
	}

	bus.deviceName = deviceName
	return nil
}

// OpenHandle lets the I2cBus type implement the board.I2C interface. It returns a handle for
// communicating with a device at a specific I2C handle. Opening a handle locks the I2C bus so
// nothing else can use it, and closing the handle unlocks the bus again.
func (bus *I2cBus) OpenHandle(addr byte) (board.I2CHandle, error) {
	bus.mu.Lock() // Lock the bus so no other handle can use it until this handle is closed.

	// If we haven't yet connected to the bus itself, do so now.
	if bus.closeableBus == nil {
		newBus, err := i2creg.Open(bus.deviceName)
		if err != nil {
			bus.mu.Unlock() // We never created a handle, so unlock the bus for next time.
			return nil, err
		}
		bus.closeableBus = newBus
	}

	return &I2cHandle{device: &i2c.Dev{Bus: bus.closeableBus, Addr: uint16(addr)}, parentBus: bus}, nil
}

// I2cHandle represents a way to talk to a specific device on the I2C bus. Creating a handle locks
// the bus so nothing else can use it, and closing the handle unlocks it again.
type I2cHandle struct { // Implements the board.I2CHandle interface
	device    *i2c.Dev // Will become nil if we Close() the handle
	parentBus *I2cBus
}

// Write writes the given bytes to the handle. For I2C devices that organize their data into
// registers, prefer using WriteBlockData instead.
func (h *I2cHandle) Write(ctx context.Context, tx []byte) error {
	return h.device.Tx(tx, nil)
}

// Read reads the given number of bytes from the handle. For I2C devices that organize their data
// into registers, prefer using ReadBlockData instead.
func (h *I2cHandle) Read(ctx context.Context, count int) ([]byte, error) {
	buffer := make([]byte, count)
	err := h.device.Tx(nil, buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

// This is a private helper function, used to implement the rest of the board.I2CHandle interface.
func (h *I2cHandle) transactAtRegister(register byte, w, r []byte) error {
	if w == nil {
		w = []byte{}
	}
	fullW := make([]byte, len(w)+1)
	fullW[0] = register
	copy(fullW[1:], w)
	return h.device.Tx(fullW, r)
}

// ReadByteData reads a single byte from the given register on this I2C device.
func (h *I2cHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	result := make([]byte, 1)
	err := h.transactAtRegister(register, nil, result)
	if err != nil {
		return 0, err
	}
	return result[0], nil
}

// WriteByteData writes a single byte to the given register on this I2C device.
func (h *I2cHandle) WriteByteData(ctx context.Context, register, data byte) error {
	return h.transactAtRegister(register, []byte{data}, nil)
}

// ReadBlockData reads the given number of bytes from the I2C device, starting at the given
// register.
func (h *I2cHandle) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	result := make([]byte, numBytes)
	err := h.transactAtRegister(register, nil, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// WriteBlockData writes the given bytes into the given register on the I2C device.
func (h *I2cHandle) WriteBlockData(ctx context.Context, register byte, data []byte) error {
	return h.transactAtRegister(register, data, nil)
}

// Close closes the handle to the device, and unlocks the I2C bus.
func (h *I2cHandle) Close() error {
	defer h.parentBus.mu.Unlock() // Unlock the entire bus so someone else can use it
	h.device = nil
	// Don't close the bus itself: it should remain open for other handles to use
	return nil
}

// GetI2CBus retrieves an I2C interface. If the bus number is specified, it uses that on the local
// machine, and otherwise it tries to get the named bus from the named board.
// TODO(RSDK-5254): remove this once all I2C devices are talking directly to the bus without going
// through the board.
func GetI2CBus(deps resource.Dependencies, boardName, busName string, busNum int) (board.I2C, error) {
	if busNum != 0 {
		return NewI2cBus(fmt.Sprintf("%d", busNum))
	}

	// Otherwise, look things up through the board.
	b, err := board.FromDependencies(deps, boardName)
	if err != nil {
		return nil, err
	}
	localBoard, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("cannot get I2C bus '%s' from nonlocal board '%s'",
			busName, boardName)
	}
	bus, success := localBoard.I2CByName(busName)
	if !success {
		return nil, fmt.Errorf("unknown I2C bus %s on board %s", busName, boardName)
	}
	return bus, nil
}
