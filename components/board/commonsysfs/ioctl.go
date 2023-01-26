package commonsysfs

// This file is heavily inspired by https://github.com/mkch/gpio

import (
	"context"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
	"go.viam.com/rdk/components/board"
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

type IoctlPin struct {
	fd     int
	offset uint32
}

// This helps implement the board.GPIOPin interface for IoctlPin.
func (pin *IoctlPin) Set(ctx context.Context, isHigh bool, extra map[string]interface{}) error {
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

// This helps implement the board.GPIOPin interface for IoctlPin.
func (pin *IoctlPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return false, nil
}

// This helps implement the board.GPIOPin interface for IoctlPin.
func (pin *IoctlPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0.0, nil
}

// This helps implement the board.GPIOPin interface for IoctlPin.
func (pin *IoctlPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	return nil
}

// This helps implement the board.GPIOPin interface for IoctlPin.
func (pin *IoctlPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	return 0, nil
}

// This helps implement the board.GPIOPin interface for IoctlPin.
func (pin *IoctlPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	return nil
}

var (
	chips map[string](*os.File)  // Maps pseudofiles from /dev to file descriptors of opened files
	pins map[string]IoctlPin
)

func ioctlInitialize(gpioMappings map[int]GPIOBoardMapping) error {
	for pin, mapping := range gpioMappings {
		file, ok := chips[mapping.GPIOChipDev]
		if !ok {
			var err error
			file, err = os.Open(fmt.Sprintf("/dev/%s", mapping.GPIOChipDev))
			if err != nil {
				return err
			}
			chips[mapping.GPIOChipDev] = file
		}
		pins[fmt.Sprintf("%d", pin)] = IoctlPin{fd: int(file.Fd()), offset: uint32(mapping.GPIO)}
	}
	return nil
}

func ioctlGetPin(pinName string) (board.GPIOPin, error) {
	pin, ok := pins[pinName]
	if !ok {
		return nil, fmt.Errorf("Cannot set GPIO for unknown pin: %s", pinName)
	}
	return &pin, nil
}
