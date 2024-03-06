package camera

import (
	"fmt"

	"github.com/bluenviron/gortsplib/v4/pkg/ringbuffer"
	"go.viam.com/rdk/logging"
)

// AsyncWriter is an asynchronous writer.
type AsyncWriter struct {
	Name   string
	logger logging.Logger
	buffer *ringbuffer.RingBuffer

	// out
	err chan error
}

// New allocates a Writer.
func NewAsyncWriter(name string, queueSize int, logger logging.Logger) *AsyncWriter {
	buffer, _ := ringbuffer.New(uint64(queueSize))

	return &AsyncWriter{
		Name:   name,
		logger: logger,
		buffer: buffer,
		err:    make(chan error),
	}
}

// Start starts the writer routine.
func (w *AsyncWriter) Start() {
	go w.run()
}

// Stop stops the writer routine.
func (w *AsyncWriter) Stop() {
	w.buffer.Close()
	<-w.err
}

// Error returns whenever there's an error.
func (w *AsyncWriter) Error() chan error {
	return w.err
}

func (w *AsyncWriter) run() {
	w.err <- w.runInner()
}

func (w *AsyncWriter) runInner() error {
	for {
		cb, ok := w.buffer.Pull()
		if !ok {
			return fmt.Errorf("terminated")
		}

		err := cb.(func() error)()
		if err != nil {
			return err
		}
	}
}

// Push appends an element to the queue.
func (w *AsyncWriter) Push(cb func() error) {
	ok := w.buffer.Push(cb)
	if !ok {
		w.logger.Warn("write queue is full")
	}
}
