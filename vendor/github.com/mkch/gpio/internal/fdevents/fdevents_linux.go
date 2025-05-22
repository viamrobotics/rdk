// Package fdevents implements epoll_wait loop for GPIO events and exposes a channel interface.
package fdevents

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Event is a GPIO event.
type Event struct {
	RisingEdge bool      // Whether this event is triggered by a rising edge.
	Time       time.Time // The best estimate of time of event occurrence.
}

type ReadFdFunc func(fd int) *Event

// FdEvents converts epoll_wait loops to a chanel.
type FdEvents struct {
	events              chan *Event
	waitLoopDone        sync.WaitGroup
	exitWaitLoopEventFd int
	closed              bool
}

// New creates a FdEvents and returns any error encountered.
// The returned FdEvents waits fd for fdEpollEvents in a epoll_wait loop.
// If epoll_wait returns successfully, readFd is called to generate a value,
// and that value is sent to the event channel returned by Events.
//
// readFd should return the best estimate of time of event occurrence.
//
// Package fdevents will not block sending to the channel: it only keeps the lastest
// value in the channel.
func New(fd int, closeFdOnClose bool, fdEpollEvents uint32, readFd ReadFdFunc) (events *FdEvents, err error) {
	wakeUpEventFd, err := unix.Eventfd(0, 0)
	if err != nil {
		err = fmt.Errorf("request GPIO event failed: eventfd: %w", err)
		return
	}

	epollFd, err := unix.EpollCreate(1)
	if err != nil {
		err = fmt.Errorf("request GPIO event failed: epoll_create: %w", err)
		unix.Close(wakeUpEventFd)
		return
	}

	err = unix.EpollCtl(epollFd, unix.EPOLL_CTL_ADD, wakeUpEventFd, &unix.EpollEvent{
		Events: unix.EPOLLIN,
		Fd:     int32(wakeUpEventFd),
	})
	if err != nil {
		err = fmt.Errorf("request GPIO event failed: epoll_ctl %w", err)
		unix.Close(wakeUpEventFd)
		unix.Close(epollFd)
		return
	}

	err = unix.EpollCtl(epollFd, unix.EPOLL_CTL_ADD, int(fd), &unix.EpollEvent{
		Events: fdEpollEvents,
		Fd:     int32(fd),
	})
	if err != nil {
		err = fmt.Errorf("request GPIO event failed: epoll_ctl %w", err)
		unix.Close(wakeUpEventFd)
		unix.Close(epollFd)
		return
	}

	events = &FdEvents{
		events:              make(chan *Event, 1), // Buffer 1 to store the latest.
		exitWaitLoopEventFd: wakeUpEventFd,
	}
	runtime.SetFinalizer(events, func(p *FdEvents) { p.Close() })

	events.waitLoopDone.Add(1)
	go events.waitLoop(fd, closeFdOnClose, epollFd, readFd)
	return
}

func (events *FdEvents) waitLoop(fd int, closeFdOnClose bool, epollFd int, readFd ReadFdFunc) {
	defer func() {
		err := unix.Close(events.exitWaitLoopEventFd)
		if err != nil {
			panic(fmt.Errorf("failed to call close: %w", err))
		}
		err = unix.Close(epollFd)
		if err != nil {
			panic(fmt.Errorf("failed to call close: %w", err))
		}
		if closeFdOnClose {
			err = unix.Close(fd)
			if err != nil {
				panic(fmt.Errorf("failed to call close: %w", err))
			}
		}
		close(events.events)
		events.waitLoopDone.Done()
	}()

	var waitEvent [2]unix.EpollEvent
epoll_wait_loop:
	for {
		n, err := unix.EpollWait(epollFd, waitEvent[:], -1)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			panic(fmt.Errorf("failed to call epoll_wait: %w", err))
		}
		for i := 0; i < n; i++ {
			switch waitEvent[i].Fd {
			case int32(fd):
				// Interrupt caused by GPIO event.
				t := readFd(fd)
				if t == nil {
					continue
				}
				// Discard the unread old value.
				select {
				case <-events.events:
				default:
				}
				// Send the latest.
				events.events <- t
			case int32(events.exitWaitLoopEventFd):
				break epoll_wait_loop
			}
		}
	}
}

func (events *FdEvents) notifyWaitLoopToExit() (err error) {
	// Wakeup epoll_wait loop adding 1 to the event counter.
	var one = uint64(1)
	n, err := unix.Write(events.exitWaitLoopEventFd, (*[unsafe.Sizeof(one)]byte)(unsafe.Pointer(&one))[:])
	if err != nil {
		err = fmt.Errorf("failed to write to event fd: %w", err)
		return
	}
	if n != int(unsafe.Sizeof(one)) {
		err = fmt.Errorf("failed to write to event fd: short write: %v out of %v", n, unsafe.Sizeof(one))
	}
	return
}

// Close stops the epoll_wait loop, close the fd, and close the event channel.
func (events *FdEvents) Close() (err error) {
	if events.closed {
		return errors.New("already closed")
	}
	events.closed = true
	err = events.notifyWaitLoopToExit()
	if err != nil {
		return
	}
	events.waitLoopDone.Wait()
	return
}

// Events returns a channel from which the occurrence time of events can be read.
// The best estimate of time of event occurrence is sent to the returned channel,
// and the channel is closed when events is closed.
//
// Package fdevents will not block sending to the channel: it only keeps the lastest
// value in the channel.
func (events *FdEvents) Events() <-chan *Event {
	return events.events
}
