// Package sys wraps necessary linux syscalls to implement GPIO interface.
package sys

import (
	"golang.org/x/sys/unix"
)

//https://github.com/torvalds/linux/blob/master/include/uapi/linux/gpio.h

const (
	GPIO_GET_CHIPINFO_IOCTL          = 0x8044b401
	GPIO_GET_LINEINFO_IOCTL          = 0xc048b402
	GPIO_GET_LINEHANDLE_IOCTL        = 0xc16cb403
	GPIOHANDLE_SET_LINE_VALUES_IOCTL = 0xc040b409
	GPIOHANDLE_GET_LINE_VALUES_IOCTL = 0xc040b408
	GPIO_GET_LINEEVENT_IOCTL         = 0xc030b404
)

// gpiochip_info
type GPIOChipInfo struct {
	Name  [32]byte
	Label [32]byte
	Lines uint32
}

const (
	GPIOLINE_FLAG_KERNEL      = 1 << 0
	GPIOLINE_FLAG_IS_OUT      = 1 << 1
	GPIOLINE_FLAG_ACTIVE_LOW  = 1 << 2
	GPIOLINE_FLAG_OPEN_DRAIN  = 1 << 3
	GPIOLINE_FLAG_OPEN_SOURCE = 1 << 4
)

// gpioline_info
type GPIOLineInfo struct {
	LineOffset uint32
	Flags      uint32
	Name       [32]byte
	Consumer   [32]byte
}

//https://www.kernel.org/doc/Documentation/gpio/gpio.txt
// https://embeddedartistry.com/blog/2018/6/4/demystifying-microcontroller-gpio-settings#open-drain-output
const (
	GPIOHANDLE_REQUEST_INPUT       = 1 << 0
	GPIOHANDLE_REQUEST_OUTPUT      = 1 << 1
	GPIOHANDLE_REQUEST_ACTIVE_LOW  = 1 << 2
	GPIOHANDLE_REQUEST_OPEN_DRAIN  = 1 << 3
	GPIOHANDLE_REQUEST_OPEN_SOURCE = 1 << 4
)

//gpiohandle_request
type GPIOHandleRequest struct {
	LineOffsets   [64]uint32
	Flags         uint32
	DefaultValues [64]byte
	ConsumerLabel [32]byte
	Lines         uint32
	Fd            int32
}

const (
	GPIOEVENT_REQUEST_RISING_EDGE  = 1 << 0
	GPIOEVENT_REQUEST_FALLING_EDGE = 1 << 1
	GPIOEVENT_REQUEST_BOTH_EDGES   = GPIOEVENT_REQUEST_RISING_EDGE | GPIOEVENT_REQUEST_FALLING_EDGE
)

// gpioevent_request
type GPIOEventRequest struct {
	LineOffset    uint32
	HandleFlags   uint32
	EventFlags    uint32
	ConsumerLabel [32]byte
	Fd            int32
}

const (
	GPIOEVENT_EVENT_RISING_EDGE  = 0x01
	GPIOEVENT_EVENT_FALLING_EDGE = 0x02
)

// gpioevent_data
type GPIOEventData struct {
	Timestamp uint64
	ID        uint32
	pad       [4]byte
}

// Ioctl call ioctl with one argument and no return value.
func Ioctl(fd int, request uintptr, a uintptr) error {
	_, _, err := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(request), a)
	if err != 0 {
		return err
	}
	return nil
}

// FdReader is a unix file descriptor as io.Reader.
type FdReader int

func (fd FdReader) Read(p []byte) (n int, err error) {
	return unix.Read(int(fd), p)
}
