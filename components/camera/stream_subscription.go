package camera

// heavily inspired by https://github.com/bluenviron/mediamtx/blob/main/internal/asyncwriter/async_writer.go

import (
	"sync"
	"sync/atomic"

	"github.com/bluenviron/gortsplib/v4/pkg/ringbuffer"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

// StreamSubscriptionID is the id of a StreamSubscription.
type StreamSubscriptionID = uuid.UUID

var (
	// ErrQueueFull indicates the StreamSubscription's queue is full and that
	// the callback passed to Publish will not be executed.
	ErrQueueFull = errors.New("StreamSubscription Publish queue full")
	// ErrClosed indicates the StreamSubscription's is not running.
	ErrClosed = errors.New("StreamSubscription has been closed")
	// ErrNegativeQueueSize indicates that the StreamSubscription queueSize
	// can't be less than 0.
	ErrNegativeQueueSize = errors.New("ErrNegativeQueueSize")
)

// StreamSubscription executes the callbacks sent to Publish
// in a single goroutine & drops Publish callbacks if the
// buffer is full.
// This is desirable behavior for streaming protocols where
// dropping stale packets is desirable to minimize latency.
type StreamSubscription struct {
	buffer  *ringbuffer.RingBuffer
	id      StreamSubscriptionID
	onError func(error)
	err     atomic.Value
	started chan struct{}
	wg      sync.WaitGroup
}

// NewStreamSubscription allocates a VideoCodecStreamSubscription.
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
		started: make(chan struct{}),
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
	<-w.started
}

// Close closes the subscription routine.
func (w *StreamSubscription) Close() {
	defer w.wg.Wait()
	w.buffer.Close()
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
	close(w.started)
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
