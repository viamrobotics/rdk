package gpio

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/mkch/gpio/internal/sys"
	"golang.org/x/sys/unix"
)

// Line is an opened GPIO line.
type Line Lines

func (l *Line) Close() (err error) {
	return (*Lines)(l).Close()
}

// Value returns the current value of the GPIO line. 1 (high) or 0 (low).
func (l *Line) Value() (value byte, err error) {
	values, err := (*Lines)(l).Values()
	if err != nil {
		return
	}
	value = values[0]
	return
}

// SetValue sets the value of the GPIO line.
// Value should be 0 (low) or 1 (high), anything else than 0 will be interpreted as 1 (high).
func (l *Line) SetValue(value byte) (err error) {
	var values = [1]byte{value}
	err = (*Lines)(l).SetValues(values[:])
	runtime.KeepAlive(values)
	return
}

// Lines is a batch of opened GPIO lines.
type Lines struct {
	fd       int
	numLines int
}

func (l *Lines) Close() (err error) {
	err = unix.Close(l.fd)
	l.fd = -1
	return
}

// Values returns the current values of the GPIO lines. 1 (high) or 0 (low).
func (l *Lines) Values() (values []byte, err error) {
	var arg [64]byte
	err = sys.Ioctl(l.fd, sys.GPIOHANDLE_GET_LINE_VALUES_IOCTL, uintptr(unsafe.Pointer(&arg[0])))
	if err != nil {
		err = fmt.Errorf("get GPIO line values failed: %w", err)
		return
	}
	values = arg[:l.numLines]
	return
}

// SetValue sets the value of the GPIO line.
// Value should be 0 (low) or 1 (high), anything else than 0 will be interpreted as 1 (high).
func (l *Lines) SetValues(values []byte) (err error) {
	if len(values) > 64 {
		err = fmt.Errorf("set GPIO line values failed: length of values(%v) > 64", len(values))
	}
	var arg [64]byte
	copy(arg[:], values)
	err = sys.Ioctl(l.fd, sys.GPIOHANDLE_SET_LINE_VALUES_IOCTL, uintptr(unsafe.Pointer(&arg[0])))
	runtime.KeepAlive(arg)
	if err != nil {
		err = fmt.Errorf("set GPIO line values failed: %w", err)
		return
	}
	return
}
