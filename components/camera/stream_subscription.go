package camera

import (
	"github.com/bluenviron/gortsplib/v4/pkg/ringbuffer"
	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// StreamSubscription executes the callbacks sent to Publish
// in a single goroutine & drops Publish callbacks if the
// buffer is full.
// This is desirable behavior for streaming protocols where
// dropping stale packets is desirable to minimize latency.
type StreamSubscription struct {
	Name   string
	logger logging.Logger
	buffer *ringbuffer.RingBuffer

	// out
	err chan error
}

// NewVideoCodecStreamSubscription allocates a VideoCodecStreamSubscription.
func NewVideoCodecStreamSubscription(
	name string,
	queueSize int,
	logger logging.Logger,
) (*StreamSubscription, error) {
	buffer, err := ringbuffer.New(uint64(queueSize))
	if err != nil {
		return nil, err
	}

	return &StreamSubscription{
		Name:   name,
		logger: logger,
		buffer: buffer,
		err:    make(chan error),
	}, nil
}

// Start starts the writer routine.
func (w *StreamSubscription) Start() {
	go w.run()
}

// Stop stops the writer routine.
func (w *StreamSubscription) Stop() {
	w.buffer.Close()
	<-w.err
}

// Error returns whenever there's an error.
func (w *StreamSubscription) Error() chan error {
	return w.err
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

		err := cb.(func() error)()
		if err != nil {
			return err
		}
	}
}

// Publish appends an element to the queue.
func (w *StreamSubscription) Publish(cb func() error) {
	ok := w.buffer.Push(cb)
	if !ok {
		w.logger.Warn("write queue is full")
	}
}
