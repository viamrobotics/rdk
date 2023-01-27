package commonsysfs

// This file is heavily inspired by https://github.com/mkch/gpio

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"sync"
	"unsafe"

	"github.com/pkg/errors"
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

func ioctl(fd uintptr, request uintptr, data uintptr) error {
	_, _, err := unix.Syscall(unix.SYS_IOCTL, fd, request, data)
	if err.Error() == "errno 0" {
		// If errno is 0, there was no error, so ignore the (lack of) problem.
		return nil
	}
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

type GPIOHandleData struct {
	Values [64]uint8
}

type ioctlPin struct {
	// These first two values should be considered immutable
	devicePath string
	offset     uint32
	mu         sync.Mutex
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) Set(ctx context.Context, isHigh bool, extra map[string]interface{}) error {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	var value byte
	if isHigh {
		value = 1
	} else {
		value = 0
	}

	devFile, err := os.Open(pin.devicePath)
	if err != nil {
		return err
	}
	defer devFile.Close()

	request := GPIOHandleRequest{LineOffsets: [64]uint32{pin.offset},
								 Flags: GPIOHANDLE_REQUEST_OUTPUT,
								 DefaultValues: [64]byte{value},
								 ConsumerLabel: [32]byte{},
								 Lines: 1,
								 Fd: 0,
								 }

	err = ioctl(devFile.Fd(), GPIO_GET_LINEHANDLE_IOCTL, uintptr(unsafe.Pointer(&request)))
	if err != nil {
	    return err
	}
	syscall.Close(int(request.Fd))
	return nil
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	devFile, err := os.Open(pin.devicePath)
	if err != nil {
		return false, err
	}
	defer devFile.Close()

	request := GPIOHandleRequest{LineOffsets: [64]uint32{pin.offset},
								 Flags: GPIOHANDLE_REQUEST_INPUT,
								 DefaultValues: [64]byte{},
								 ConsumerLabel: [32]byte{},
								 Lines: 1,
								 Fd: 0,
								 }

	err = ioctl(devFile.Fd(), GPIO_GET_LINEHANDLE_IOCTL, uintptr(unsafe.Pointer(&request)))
	if err != nil {
	    return false, err
	}
	defer syscall.Close(int(request.Fd))

	readRequest := GPIOHandleData{Values: [64]uint8{}}
	err = ioctl(uintptr(request.Fd), GPIOHANDLE_GET_LINE_VALUES_IOCTL,
				uintptr(unsafe.Pointer(&readRequest)))
	if err != nil {
	    return false, err
	}

	return (readRequest.Values[0] != 0), nil
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0.0, errors.New("PWM stuff is not supported on ioctl pins yet")
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	return errors.New("PWM stuff is not supported on ioctl pins yet")
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	return 0, errors.New("PWM stuff is not supported on ioctl pins yet")
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	return errors.New("PWM stuff is not supported on ioctl pins yet")
}

var (
	pins map[string]ioctlPin
)

func ioctlInitialize(gpioMappings map[int]GPIOBoardMapping) {
	pins = make(map[string]ioctlPin)
	for pin, mapping := range gpioMappings {
		pins[fmt.Sprintf("%d", pin)] = ioctlPin{
			devicePath: fmt.Sprintf("/dev/%s", mapping.GPIOChipDev),
			offset: uint32(mapping.GPIO),
		}
	}
}

func ioctlGetPin(pinName string) (board.GPIOPin, error) {
	pin, ok := pins[pinName]
	if !ok {
		return nil, errors.Errorf("Cannot set GPIO for unknown pin: %s", pinName)
	}
	return &pin, nil
}
