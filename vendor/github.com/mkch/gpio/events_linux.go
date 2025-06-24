package gpio

import (
	"fmt"
	"io"
	"syscall"
	"time"
	"unsafe"

	"github.com/mkch/gpio/internal/fdevents"
	"github.com/mkch/gpio/internal/sys"
	"golang.org/x/sys/unix"
)

// LineWithEvent is an opened GPIO line whose events can be subscribed.
type LineWithEvent struct {
	l      Line
	events *fdevents.FdEvents
}

func (l *LineWithEvent) Close() (err error) {
	// Close l.events first because Line.Close will close the fd,
	// but l.events still needs it until it is Closed.
	// l.events.Close won't close the fd. See newInputLineWithEvents.
	err1 := l.events.Close()
	err2 := l.l.Close()
	if err1 != nil {
		return err1
	}
	return err2

}

// Value returns the current value of the GPIO line. 1 (high) or 0 (low).
func (l *LineWithEvent) Value() (value byte, err error) {
	return l.l.Value()
}

func readGPIOLineEventFd(fd int) *fdevents.Event {
	var eventData sys.GPIOEventData
	_, err := io.ReadFull(sys.FdReader(fd), (*[unsafe.Sizeof(eventData)]byte)(unsafe.Pointer(&eventData))[:])
	if err != nil {
		if err == syscall.EINTR {
			return nil // ignore
		}
		panic(fmt.Errorf("failed to read GPIO event: %w", err))
	}

	sec := uint64(time.Nanosecond) * eventData.Timestamp / uint64(time.Second)
	nano := uint64(time.Nanosecond) * eventData.Timestamp % uint64(time.Second)
	return &fdevents.Event{RisingEdge: eventData.ID == sys.GPIOEVENT_EVENT_RISING_EDGE, Time: time.Unix(int64(sec), int64(nano))}
}

func newInputLineWithEvents(chipFd int, offset uint32, flags, eventFlags uint32, consumer string) (line *LineWithEvent, err error) {
	var req = sys.GPIOEventRequest{
		LineOffset:  offset,
		HandleFlags: uint32(flags),
		EventFlags:  uint32(eventFlags)}
	copy(req.ConsumerLabel[:], consumer)
	err = sys.Ioctl(chipFd, sys.GPIO_GET_LINEEVENT_IOCTL, uintptr(unsafe.Pointer(&req)))
	if err != nil {
		err = fmt.Errorf("request GPIO event failed: ioctl %w", err)
		return
	}
	events, err := fdevents.New(int(req.Fd), false /*NOT close fd*/, unix.EPOLLIN|unix.EPOLLPRI, readGPIOLineEventFd)
	if err != nil {
		unix.Close(int(req.Fd))
		return
	}

	line = &LineWithEvent{
		l:      Line{fd: int(req.Fd), numLines: 1},
		events: events,
	}
	return
}

type Event = fdevents.Event

// Events returns a channel from which the occurrence time of GPIO events can be read.
// The GPIO events of this line will be sent to the returned channel,
// and the channel is closed when l is closed.
//
// Package gpio will not block sending to the channel: it only keeps the lastest
// value in the channel.
func (l *LineWithEvent) Events() <-chan *Event {
	return l.events.Events()
}
