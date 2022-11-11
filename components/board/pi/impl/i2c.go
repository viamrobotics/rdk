//go:build linux && (arm64 || arm)

package piimpl

// #include <stdlib.h>
// #include <pigpio.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
)

type piPigpioI2C struct {
	pi *piPigpio
	id int
}

type piPigpioI2CHandle struct {
	bus      *piPigpioI2C
	i2cFlags C.uint
	handle   C.uint
}

// Write will write the given slice of bytes to the given i2c address
func (s *piPigpioI2CHandle) Write(ctx context.Context, tx []byte) error {
	txPtr := C.CBytes(tx)
	defer C.free(txPtr)

	ret := C.i2cWriteDevice(s.handle, (*C.char)(txPtr), (C.uint)(len(tx)))

	if int(ret) != 0 {
		return errors.Errorf("error with i2c write %q", ret)
	}

	return nil
}

// Read will read `count` bytes from the given address.
func (s *piPigpioI2CHandle) Read(ctx context.Context, count int) ([]byte, error) {
	rx := make([]byte, count)
	rxPtr := C.CBytes(rx)
	defer C.free(rxPtr)

	ret := C.i2cReadDevice(s.handle, (*C.char)(rxPtr), (C.uint)(count))

	if int(ret) <= 0 {
		return nil, errors.Errorf("error with i2c read %q", ret)
	}

	return C.GoBytes(rxPtr, (C.int)(count)), nil
}

func (s *piPigpioI2CHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	res := C.i2cReadByteData(s.handle, C.uint(register))
	if res < 0 {
		return 0, errors.Errorf("error in ReadByteData (%d)", res)
	}
	return byte(res & 0xFF), nil
}

func (s *piPigpioI2CHandle) WriteByteData(ctx context.Context, register, data byte) error {
	res := C.i2cWriteByteData(s.handle, C.uint(register), C.uint(data))
	if res != 0 {
		return errors.Errorf("error in WriteByteData (%d)", res)
	}
	return nil
}

func (s *piPigpioI2CHandle) ReadWordData(ctx context.Context, register byte) (uint16, error) {
	res := C.i2cReadWordData(s.handle, C.uint(register))
	if res < 0 {
		return 0, errors.Errorf("error in ReadWordData (%d)", res)
	}
	return uint16(res & 0xFFFF), nil
}

func (s *piPigpioI2CHandle) WriteWordData(ctx context.Context, register byte, data uint16) error {
	res := C.i2cWriteWordData(s.handle, C.uint(register), C.uint(data))
	if res != 0 {
		return errors.Errorf("error in WriteWordData (%d)", res)
	}
	return nil
}

func (s *piPigpioI2CHandle) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	if numBytes > 32 { // A limitation from the underlying pigpio.h library
		return nil, errors.New("Cannot read more than 32 bytes from I2C")
	}

	data := make([]byte, numBytes)
	response := C.i2cReadI2CBlockData(
		s.handle, C.uint(register), (*C.char)(&data[0]), C.uint(numBytes))
	if response < 0 {
		return nil, errors.Errorf("error in ReadBlockData (%d)", response)
	}
	return data, nil
}

func (s *piPigpioI2CHandle) WriteBlockData(ctx context.Context, register byte, numBytes uint8, data []byte) error {
	if numBytes > 32 { // A limitation from the underlying pigpio.h library
		return errors.New("Cannot write more than 32 bytes from I2C")
	}

	response := C.i2cWriteI2CBlockData(
		s.handle, C.uint(register), (*C.char)(&data[0]), C.uint(numBytes))
	if response != 0 {
		return errors.Errorf("error in WriteBlockData (%d)", response)
	}
	return nil
}

func (s *piPigpioI2C) OpenHandle(addr byte) (board.I2CHandle, error) {
	handle := &piPigpioI2CHandle{bus: s}

	// Raspberry Pis are all on i2c bus 1
	// Exception being the very first model which is on 0
	bus := (C.uint)(s.id)
	temp := C.i2cOpen(bus, (C.uint)(addr), handle.i2cFlags)

	if temp < 0 {
		return nil, errors.Errorf("error opening I2C Bus %d return code was %d, flags were %X", bus, temp, handle.i2cFlags)
	}
	handle.handle = C.uint(temp)

	return handle, nil
}

func (s *piPigpioI2CHandle) Close() error {
	C.i2cClose(s.handle)
	return nil
}
