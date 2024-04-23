package rtppassthrough

// heavily inspired by https://github.com/bluenviron/mediamtx/blob/main/internal/asyncwriter/async_writer.go

// NOTE: (Nick S)
// StreamSubscription is what powers camera.SubscribeRTP.
// It runs a single goroutine which pulls and runs callback functions
// in order published from a fixed capacity ringbuffer.
// When the ringbuffer's capacity is reached, `Publish` returns an error.
// This is important to maintain a bounded
// amount of queuing as stale video stream packets degrade video quality.

// At time of writing
// One callback function is created & executed per RTP packet per subscribed client with the nuance that:
// (at time of writing) gostream is handling multiplexing from 1 track (a camera video feed) to N web
// browser peer connections interested in that track: https://github.com/viamrobotics/rdk/blob/main/gostream/webrtc_track.go#L126
// As a result, at time of writing StreamSubscriptions only ever have a single publisher and a single subscriber.

import (
	"sync"
	"sync/atomic"

	"errors"

	"github.com/bluenviron/gortsplib/v4/pkg/ringbuffer"
	"github.com/google/uuid"
	"go.viam.com/utils"
)

// SubscriptionID is the id of a StreamSubscription.
type SubscriptionID = uuid.UUID

var (
	// ErrQueueFull indicates the StreamSubscription's queue is full and that
	// the callback passed to Publish will not be executed.
	ErrQueueFull = errors.New("StreamSubscription Publish queue full")
	// ErrClosed indicates the StreamSubscription's is not running.
	ErrClosed = errors.New("StreamSubscription has been closed")
	// ErrNegativeQueueSize indicates that the StreamSubscription queueSize
	// can't be less than 0.
	ErrNegativeQueueSize = errors.New("StreamSubscription queue size can't be negative")
)

// StreamSubscription executes the callbacks sent to Publish
// in a single goroutine & drops Publish callbacks if the
// buffer is full.
// This is desirable behavior for streaming protocols where
// dropping stale packets is desirable to minimize latency.
type StreamSubscription struct {
	buffer  *ringbuffer.RingBuffer
	id      SubscriptionID
	onError func(error)
	err     atomic.Value
	wg      sync.WaitGroup
}

// NewStreamSubscription allocates an rtp passthrough stream subscription.
func NewStreamSubscription(queueSize int, onError func(error)) (*StreamSubscription, error) {
	if queueSize < 0 {
		return nil, ErrNegativeQueueSize
	}
	buffer, err := ringbuffer.New(uint64(queueSize))
	if err != nil {
		return nil, err
	}

	return &StreamSubscription{
		buffer:  buffer,
		id:      uuid.New(),
		onError: onError,
	}, nil
}

// ID returns the id of the StreamSubscription.
func (w *StreamSubscription) ID() uuid.UUID {
	return w.id
}

// Start starts the subscription routine.
func (w *StreamSubscription) Start() {
	w.wg.Add(1)
	utils.ManagedGo(w.run, w.wg.Done)
}

// Close closes the subscription routine.
func (w *StreamSubscription) Close() {
	w.buffer.Close()
	w.wg.Wait()
}

// Publish publishes the callback to the subscriber
// If there are too many queued messages
// return an error and does not publish.
func (w *StreamSubscription) Publish(cb func() error) error {
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

func (w *StreamSubscription) run() {
	if err := w.runInner(); err != nil && w.onError != nil {
		w.onError(err)
	}
}

func (w *StreamSubscription) runInner() error {
	for {
		cb, ok := w.buffer.Pull()
		if !ok {
			w.err.Store(ErrClosed)
			return nil
		}

		err := cb.(func() error)()
		if err != nil {
			w.err.Store(err)
			return err
		}
	}
}
