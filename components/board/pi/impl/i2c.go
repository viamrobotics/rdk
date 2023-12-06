//go:build linux && (arm64 || arm) && !no_pigpio && !no_cgo

package piimpl

// #include <stdlib.h>
// #include <pigpio.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	picommon "go.viam.com/rdk/components/board/pi/common"
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

	ret := int(C.i2cWriteDevice(s.handle, (*C.char)(txPtr), (C.uint)(len(tx))))

	if ret != 0 {
		return picommon.ConvertErrorCodeToMessage(ret, "error with i2c write")
	}

	return nil
}

// Read will read `count` bytes from the given address.
func (s *piPigpioI2CHandle) Read(ctx context.Context, count int) ([]byte, error) {
	rx := make([]byte, count)
	rxPtr := C.CBytes(rx)
	defer C.free(rxPtr)

	ret := int(C.i2cReadDevice(s.handle, (*C.char)(rxPtr), (C.uint)(count)))

	if ret <= 0 {
		return nil, picommon.ConvertErrorCodeToMessage(ret, "error with i2c read")
	}

	return C.GoBytes(rxPtr, (C.int)(count)), nil
}

func (s *piPigpioI2CHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	res := C.i2cReadByteData(s.handle, C.uint(register))
	if res < 0 {
		return 0, picommon.ConvertErrorCodeToMessage(int(res), "error in ReadByteData")
	}
	return byte(res & 0xFF), nil
}

func (s *piPigpioI2CHandle) WriteByteData(ctx context.Context, register, data byte) error {
	res := C.i2cWriteByteData(s.handle, C.uint(register), C.uint(data))
	if res != 0 {
		return picommon.ConvertErrorCodeToMessage(int(res), "error in WriteByteData")
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
		return nil, picommon.ConvertErrorCodeToMessage(int(response), "error in ReadBlockData")
	}
	return data, nil
}

func (s *piPigpioI2CHandle) WriteBlockData(ctx context.Context, register byte, data []byte) error {
	numBytes := len(data)
	if numBytes > 32 { // A limitation from the underlying pigpio.h library
		return errors.New("Cannot write more than 32 bytes from I2C")
	}

	response := C.i2cWriteI2CBlockData(
		s.handle, C.uint(register), (*C.char)(&data[0]), C.uint(numBytes))
	if response != 0 {
		return picommon.ConvertErrorCodeToMessage(int(response), "error in WriteBlockData")
	}
	return nil
}

func (s *piPigpioI2C) OpenHandle(addr byte) (buses.I2CHandle, error) {
	handle := &piPigpioI2CHandle{bus: s}

	// Raspberry Pis are all on i2c bus 1
	// Exception being the very first model which is on 0
	bus := (C.uint)(s.id)
	temp := C.i2cOpen(bus, (C.uint)(addr), handle.i2cFlags)

	if temp < 0 {
		errMsg := fmt.Sprintf("error opening I2C Bus %d, flags were %X", bus, handle.i2cFlags)
		return nil, picommon.ConvertErrorCodeToMessage(int(temp), errMsg)
	}
	handle.handle = C.uint(temp)

	return handle, nil
}

func (s *piPigpioI2CHandle) Close() error {
	C.i2cClose(s.handle)
	return nil
}
