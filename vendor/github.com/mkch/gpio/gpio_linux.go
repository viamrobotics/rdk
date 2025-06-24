package gpio

import (
	"fmt"
	"path/filepath"
	"unsafe"

	"github.com/mkch/gpio/internal/sys"
	"golang.org/x/sys/unix"
)

// ChipDevices returns all available GPIO chip devices.
// The returned paths can be used to call OpenChip.
func ChipDevices() (chips []string) {
	chips, err := filepath.Glob("/dev/gpiochip*")
	if err != nil {
		// The only possible returned error is ErrBadPattern, when pattern is malformed.
		panic(err)
	}
	for i, dev := range chips {
		chips[i] = filepath.Base(dev)
	}
	return
}

// ChipInfo is the information about a certain GPIO chip.
type ChipInfo struct {
	// The Linux kernel name of this GPIO chip.
	Name string
	// A functional name for this GPIO chip, such as a product number, may be empty.
	Label string
	// Number of GPIO lines on this chip.
	NumLines uint32
}

// Chip is certain GPIO chip.
type Chip struct {
	dev string
	fd  int
}

// OpenChip opens a certain GPIO chip device.
func OpenChip(device string) (chip *Chip, err error) {
	devPath := filepath.Join("/dev", device)
	fd, err := unix.Open(devPath, unix.O_RDONLY, 0)
	if err != nil {
		err = fmt.Errorf("open chip %v failed: %w", devPath, err)
		return
	}
	chip = &Chip{dev: device, fd: fd}
	return
}

func (c *Chip) Close() (err error) {
	err = unix.Close(c.fd)
	c.fd = -1
	return
}

// Info returns the information of this GPIO chip.
func (c *Chip) Info() (info ChipInfo, err error) {
	var arg sys.GPIOChipInfo
	err = sys.Ioctl(c.fd, sys.GPIO_GET_CHIPINFO_IOCTL, uintptr(unsafe.Pointer(&arg)))
	if err != nil {
		err = fmt.Errorf("get GPIO chip info of %s failed: %w", c.dev, err)
		return
	}
	info = ChipInfo{
		Name:     sys.Str32(arg.Name),
		Label:    sys.Str32(arg.Label),
		NumLines: arg.Lines,
	}
	return
}

// LineInfo returns the information about a certain GPIO line.
// Offset is the local line offset on this GPIO chip.
func (c *Chip) LineInfo(offset uint32) (info LineInfo, err error) {
	var arg = sys.GPIOLineInfo{LineOffset: offset}
	err = sys.Ioctl(c.fd, sys.GPIO_GET_LINEINFO_IOCTL, uintptr(unsafe.Pointer(&arg)))
	if err != nil {
		err = fmt.Errorf("get GPIO line info %v %v failed: %w", c.dev, offset, err)
		return
	}
	info = LineInfo{
		Offset:   arg.LineOffset,
		Name:     sys.Str32(arg.Name),
		Consumer: sys.Str32(arg.Consumer),
		flags:    arg.Flags,
	}
	return
}

// openLines opens up to 64 lines on this GPIO chip at once.
// Offsets are the local line offsets on this chip.
// DefaultValues are the values set to lines, value should be 0 (low) or 1 (high),
// anything else than 0 or 1 will be interpreted as 1 (high).
// If there are more values than requested lines, the extra values will be discarded. If there are less values,
// the missing values will be 0.
// Flags are or'ed LineFlag values that will be applied to all quested lines.
// Consumer is a desired consumer label for the selected GPIO line(s) such as "my-bitbanged-relay".
func (c *Chip) requestLines(offsets []uint32, outputDefaultValues []byte, requestFlags uint32, consumer string) (result *Lines, err error) {
	if len(offsets) > 64 {
		err = fmt.Errorf("open GPIO lines failed: length of offsets(%v) > 64", len(offsets))
	}
	if len(outputDefaultValues) > 64 {
		err = fmt.Errorf("open GPIO lines failed: length of default values(%v) > 64", len(outputDefaultValues))
	}
	var numLines = len(offsets)
	var arg = sys.GPIOHandleRequest{
		Flags: uint32(requestFlags),
		Lines: uint32(numLines),
	}
	copy(arg.LineOffsets[:], offsets)
	copy(arg.DefaultValues[:], outputDefaultValues)
	arg.ConsumerLabel = sys.Char32(consumer)

	err = sys.Ioctl(c.fd, sys.GPIO_GET_LINEHANDLE_IOCTL, uintptr(unsafe.Pointer(&arg)))
	if err != nil {
		err = fmt.Errorf("open GPIO lines %v on %v failed: %w", offsets, c.dev, err)
		return
	}

	result = &Lines{fd: int(arg.Fd), numLines: numLines}
	return
}

type LineFlag uint32

const (
	// Direction input.
	Input = LineFlag(sys.GPIOHANDLE_REQUEST_INPUT)
	// Direction output.
	Output = LineFlag(sys.GPIOHANDLE_REQUEST_OUTPUT)
	// ActiveLow inverts the value for writing.
	ActiveLow LineFlag = LineFlag(sys.GPIOHANDLE_REQUEST_ACTIVE_LOW)
	// OpenDrain
	//
	// https://embeddedartistry.com/blog/2018/6/4/demystifying-microcontroller-gpio-settings#open-drain-output=
	// "Unlike push-pull, an open-drain output can only sink current. The output has two states: low and high-impedance.
	// In order to achieve a logical high output on the line, a pull-up resistor is used to connect the open-drain output
	// to the desired output voltage level.
	// You can think of an open-drain GPIO as behaving like a switch which is either connected to ground or disconnected."
	OpenDrain  = LineFlag(sys.GPIOHANDLE_REQUEST_OPEN_DRAIN)
	OpenSource = LineFlag(sys.GPIOHANDLE_REQUEST_OPEN_SOURCE)
)

// OpenLines opens up to 64 lines on this GPIO chip at once.
// Parameter offsets are the local line offsets on this chip.
// Parameter defaultValues specifies the default output values,
// if Output is set in flags. Value should be 0 (low) or 1 (high),
// anything else than 0 be interpreted as 1 (high).
// DefaultValues is ignored if Output is not set in flags.
// If there are more default values than requested output lines, the extra values
// will be discarded, and if there are less values, the missing values will be 0s.
// Parameter flags is or'ed LineFlag values that will be applied to all quested lines.
// Parameter consumer is a desired consumer label for the selected GPIO line(s) such
// as "my-bitbanged-relay".
func (c *Chip) OpenLines(offsets []uint32, defaultValues []byte, flags LineFlag, consumer string) (*Lines, error) {
	return c.requestLines(offsets, defaultValues, uint32(flags), consumer)
}

// OpenLine opens a single GPIO line on this chip.
// It is equivalent to call OpenLines with a single offset and devault value if Output
// is set.
func (c *Chip) OpenLine(offset uint32, defaultValue byte, flags LineFlag, consumer string) (line *Line, err error) {
	var offsets = [1]uint32{offset}
	var defaultValues = [1]byte{defaultValue}
	lines, err := c.requestLines(offsets[:], defaultValues[:], uint32(flags), consumer)
	if err != nil {
		return
	}
	line = (*Line)(lines)
	return
}

type EventFlag uint32

const (
	RisingEdge  EventFlag = EventFlag(sys.GPIOEVENT_REQUEST_RISING_EDGE)
	FallingEdge           = EventFlag(sys.GPIOEVENT_EVENT_FALLING_EDGE)
	BothEdges             = RisingEdge | FallingEdge
)

// OpenLineWithEvents opens a single GPIO line on this chip for input and GPIO events.
func (c *Chip) OpenLineWithEvents(offset uint32, flags LineFlag, eventFlags EventFlag, consumer string) (line *LineWithEvent, err error) {
	if eventFlags == 0 {
		err = fmt.Errorf("open GPIO line failed: invalid event flags %v, at least one edge is required", eventFlags)
		return
	}
	return newInputLineWithEvents(c.fd, offset, uint32(flags), uint32(eventFlags), consumer)
}

// LineInfo represents the information about a certain GPIO line
type LineInfo struct {
	// The offset of this line on the chip.
	Offset uint32
	// The name of this GPIO line, such as the output pin of the line on the
	// chip, a rail or a pin header name on a board, as specified by the gpio
	// chip, may be empty.
	Name string
	// A functional name for the consumer of this GPIO line as set by
	// whatever is using it, will be empty if there is no current user but may
	// also be empty if the consumer doesn't set this up.
	Consumer string
	flags    uint32
}

// Kernel returns whether the GPIO line is used by the kernel.
func (info *LineInfo) Kernel() bool {
	return info.flags&sys.GPIOLINE_FLAG_KERNEL != 0
}

// Output returns whether the GPIO line is output.
func (info *LineInfo) Output() bool {
	return info.flags&sys.GPIOLINE_FLAG_IS_OUT != 0
}

// ActiveLow returns whether the GPIO line is configured as active low.
func (info *LineInfo) ActiveLow() bool {
	return info.flags&sys.GPIOLINE_FLAG_ACTIVE_LOW != 0
}

// ActiveLow returns whether the GPIO line is configured as open-drain.
func (info *LineInfo) OpenDrain() bool {
	return info.flags&sys.GPIOLINE_FLAG_OPEN_DRAIN != 0
}

// ActiveLow returns whether the GPIO line is configured as open-source.
func (info *LineInfo) OpenSource() bool {
	return info.flags&sys.GPIOLINE_FLAG_OPEN_SOURCE != 0
}
