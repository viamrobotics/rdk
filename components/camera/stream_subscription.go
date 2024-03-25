package camera

import (
	"github.com/bluenviron/gortsplib/v4/pkg/ringbuffer"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

type StreamSubscriptionID = uuid.UUID

// StreamSubscription executes the callbacks sent to Publish
// in a single goroutine & drops Publish callbacks if the
// buffer is full.
// This is desirable behavior for streaming protocols where
// dropping stale packets is desirable to minimize latency.
type StreamSubscription struct {
	id     StreamSubscriptionID
	buffer *ringbuffer.RingBuffer
	err    chan error
}

// NewStreamSubscription allocates a VideoCodecStreamSubscription.
func NewStreamSubscription(queueSize int) (*StreamSubscription, error) {
	buffer, err := ringbuffer.New(uint64(queueSize))
	if err != nil {
		return nil, err
	}

	return &StreamSubscription{
		id:     uuid.New(),
		buffer: buffer,
		err:    make(chan error),
	}, nil
}

// Start starts the writer routine.
func (w *StreamSubscription) ID() uuid.UUID {
	return w.id
}

// Start starts the writer routine.
func (w *StreamSubscription) Start() {
	utils.PanicCapturingGo(w.run)
}

// Stop stops the writer routine.
func (w *StreamSubscription) Stop() {
	w.buffer.Close()
	<-w.err
}

func (w *StreamSubscription) run() {
	w.err <- w.runInner()
}

func (w *StreamSubscription) runInner() error {
	for {
		cb, ok := w.buffer.Pull()
		if !ok {
			return errors.New("terminated")
		}

		// TODO: Test that this means that if there is the callback returns an error
		// it will deregister the subscriber & leave the goroutine alive
		// and blocked on writing to w.err until
		// Stop() is called
		err := cb.(func() error)()
		if err != nil {
			return err
		}
	}
}

// Publish publishes the callback to the subscriber
// return an error and does not publish
// if there are too many queued messages to publish.
func (w *StreamSubscription) Publish(cb func() error) error {
	ok := w.buffer.Push(cb)
	if !ok {
		return errors.New("StreamSubscription Publish queue is full")
	}
	return nil
}
