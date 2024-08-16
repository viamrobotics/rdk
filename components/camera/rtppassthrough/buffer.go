package rtppassthrough

// heavily inspired by https://github.com/bluenviron/mediamtx/blob/main/internal/asyncwriter/async_writer.go

// NOTE: (Nick S)
// *Buffer is what powers camera.SubscribeRTP.
// It runs a single goroutine which pulls and runs callback functions
// in order published from a fixed capacity ringbuffer.
// When the ringbuffer's capacity is reached, `Publish` returns an error.
// This is important to maintain a bounded
// amount of queuing as stale video stream packets degrade video quality.

// One callback function is created & executed per RTP packet per subscribed client with the nuance that:
// (at time of writing) gostream is handling multiplexing from 1 track (a camera video feed) to N web
// browser peer connections interested in that track: https://github.com/viamrobotics/rdk/blob/main/gostream/webrtc_track.go#L126
// As a result, at time of writing, a *Buffer only ever has a single publisher and a single subscriber.

// NOTE: At time of writing github.com/bluenviron/gortsplib/v4/pkg/ringbuffer is not a ringbuffer (despite the name).
// It drops the newest data when full, not the oldest.

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/bluenviron/gortsplib/v4/pkg/ringbuffer"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

var (
	// ErrQueueFull indicates the Buffer's queue is full and that
	// the callback passed to Publish will not be executed.
	ErrQueueFull = errors.New("Buffer Publish queue full")
	// ErrClosed indicates the Buffer is not running.
	ErrClosed = errors.New("Buffer has been closed")
	// ErrBufferSize indicates that the Buffer size
	// can't be less than 0.
	ErrBufferSize = errors.New("Buffer size can't be negative")
)

// Buffer executes the callbacks sent to Publish
// in a single goroutine & drops Publish callbacks if the
// buffer is full.
// This is desirable behavior for streaming protocols where
// dropping stale packets is desirable to minimize latency.
type Buffer struct {
	terminatedFn context.CancelFunc
	buffer       *ringbuffer.RingBuffer
	err          atomic.Value
	wg           sync.WaitGroup
}

// NewSubscription allocates an rtppassthrough *Buffer and
// a Subscription.
// The *Buffer is intended to be used by the rtppassthrough.Source
// implemnter. The Subscription is intended to be returned
// to the SubscribeRTP caller (aka the subscriber).
// When the Subscription has terminated, the rtppassthrough.Source
// implemnter should call Close() on the *Buffer to notify the subscriber
// that the subscription has terminated.
func NewSubscription(size int) (Subscription, *Buffer, error) {
	if size < 0 {
		return NilSubscription, nil, ErrBufferSize
	}
	buffer, err := ringbuffer.New(uint64(size))
	if err != nil {
		return NilSubscription, nil, err
	}

	terminated, terminatedFn := context.WithCancel(context.Background())
	return Subscription{ID: uuid.New(), Terminated: terminated},
		&Buffer{terminatedFn: terminatedFn, buffer: buffer},
		nil
}

// Start starts the buffer goroutine.
func (w *Buffer) Start() {
	w.wg.Add(1)
	utils.ManagedGo(w.run, w.wg.Done)
}

// Close closes the buffer goroutine
// and terminates the Subscription.
func (w *Buffer) Close() {
	w.buffer.Close()
	w.wg.Wait()
	w.terminatedFn()
}

// Publish publishes adds the callback to the buffer
// where it will be run in the future.
// If the buffer is full, it returnns an error and does
// add the callback to the buffer.
//
// Dan: Public rtp.Packets and not callbacks? Until we've proved a need for more generality?
func (w *Buffer) Publish(cb func()) error {
	rawErr := w.err.Load()

	if err, ok := rawErr.(error); ok && err != nil {
		return err
	}
	ok := w.buffer.Push(cb)
	if !ok {
		return ErrQueueFull
	}
	return nil
}

func (w *Buffer) run() {
	for {
		cb, ok := w.buffer.Pull()
		if !ok {
			w.err.Store(ErrClosed)
			return
		}

		cb.(func())()
	}
}
