package commonsysfs

// This file is heavily inspired by https://github.com/mkch/gpio

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	GPIO_GET_CHIPINFO_IOCTL          = 0x8044b401
	GPIO_GET_LINEINFO_IOCTL          = 0xc048b402
	GPIO_GET_LINEHANDLE_IOCTL        = 0xc16cb403
	GPIOHANDLE_SET_LINE_VALUES_IOCTL = 0xc040b409
	GPIOHANDLE_GET_LINE_VALUES_IOCTL = 0xc040b408
	GPIO_GET_LINEEVENT_IOCTL         = 0xc030b404

	GPIOHANDLE_REQUEST_INPUT       = 1 << 0
	GPIOHANDLE_REQUEST_OUTPUT      = 1 << 1
	GPIOHANDLE_REQUEST_ACTIVE_LOW  = 1 << 2
	GPIOHANDLE_REQUEST_OPEN_DRAIN  = 1 << 3
	GPIOHANDLE_REQUEST_OPEN_SOURCE = 1 << 4
)

func ioctl(fd int, request uintptr, data uintptr) error {
	_, _, err := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), request, data)
	return err
}

type ioctlPin struct {
	fd     int
	offset uint32
}

/*
type GPIOChipInfo struct {
	Name  [32]byte
	Label [32]byte
	Lines uint32
}

type GPIOLineInfo struct {
	LineOffset uint32
	Flags      uint32
	Name       [32]byte
	Consumer   [32]byte
}
*/

type GPIOHandleRequest struct {
	LineOffsets   [64]uint32
	Flags         uint32
	DefaultValues [64]byte
	ConsumerLabel [32]byte
	Lines         uint32
	Fd            int32
}

func (pin *ioctlPin) Set(isHigh bool) error {
	var value byte
	if isHigh {
		value = 1
	} else {
		value = 0
	}
	request := GPIOHandleRequest{LineOffsets: [64]uint32{pin.offset},
								Flags: GPIOHANDLE_REQUEST_OUTPUT,
								DefaultValues: [64]byte{value},
								Lines: 1,
								}
	return ioctl(pin.fd, GPIO_GET_LINEHANDLE_IOCTL, uintptr(unsafe.Pointer(&request)))
}
