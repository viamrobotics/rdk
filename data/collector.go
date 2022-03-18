// Package data contains the code involved with Viam's Data Management Platform for automatically collecting component
// readings from robots.
package data

import (
	"bufio"
	"context"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"google.golang.org/protobuf/types/known/structpb"
	"os"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
)

// queueSize defines the size of Collector's queue. It should be big enough to ensure that .capture() is never blocked
// by the queue being written to disk. A default value of 25 was chosen because even with the fastest reasonable capture
// interval (10ms), this would leave 250ms for a (buffered) disk write before blocking, which seems sufficient for the
// size of writes this would be performing.
const queueSize = 25

// Capturer provides a function for capturing a single protobuf reading from the underlying component.
type Capturer interface {
	Capture(ctx context.Context, params map[string]string) (interface{}, error)
}

// Collector collects data to some target.
type Collector interface {
	SetTarget(file *os.File)
	GetTarget() *os.File
	Close()
	Collect() error
}

type collector struct {
	queue     chan *structpb.Struct
	interval  time.Duration
	params    map[string]string
	lock      *sync.Mutex
	logger    golog.Logger
	target    *os.File
	writer    *bufio.Writer
	cancelCtx context.Context
	cancel    context.CancelFunc
	capturer  Capturer
}

// SetTarget updates the file being written to by the collector.
func (c *collector) SetTarget(file *os.File) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.target = file
	if err := c.writer.Flush(); err != nil {
		c.logger.Errorw("failed to flush writer to disk", "error", err)
	}
	c.writer = bufio.NewWriter(file)
}

// GetTarget returns the file being written to by the collector.
func (c *collector) GetTarget() *os.File {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.target
}

// Close closes the channels backing the Collector. It should always be called before disposing of a Collector to avoid
// leaking goroutines. Close() can only be called once; attempting to Close an already closed Collector will panic.
func (c *collector) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Must close c.queue before calling c.writer.Flush() to ensure that all captures have made it into the buffer
	// before it is flushed. Otherwise, some reading might still be queued but not yet written, and would be lost.
	c.cancel()
	close(c.queue)

	if err := c.writer.Flush(); err != nil {
		c.logger.Errorw("failed to flush writer to disk", "error", err)
	}
}

// TODO: Decide on error behavior here. Should receiving a single error from c.capture cause this to return an error?
//       I think the approach here will be more well informed when we start implementing the Data Manager Service and
//       actually using Collectors. I'm going to leave the behavior here for then. As is, I'll leave it just logging
//       errors.

// Collect starts the Collector, causing it to run c.capture every c.interval, and write the results to c.target.
func (c *collector) Collect() error {
	errs, _ := errgroup.WithContext(c.cancelCtx)
	go c.capture()
	errs.Go(func() error {
		return c.write()
	})
	return errs.Wait()
}

func (c *collector) capture() {
	ticker := time.NewTicker(c.interval)
	for {
		select {
		case <-c.cancelCtx.Done():
			return
		case <-ticker.C:
			reading, err := c.capturer.Capture(c.cancelCtx, c.params)
			if err != nil {
				c.logger.Errorw("error while capturing data", "error", err)
				break
			}
			msg, err := InterfaceToStruct(reading)
			if err != nil {
				c.logger.Errorw("error while converting reading to structpb.Struct", "error", err)
				break
			}
			select {
			// If c.queue is full, c.queue <- a can block indefinitely. This additional select block allows cancel to
			// still work when this happens.
			case <-c.cancelCtx.Done():
				return
			case c.queue <- msg:
				break
			}
		}
	}
}

// NewCollector returns a new Collector with the passed capturer and configuration options. It calls capturer at the
// specified Interval, and appends the resulting reading to target.
func NewCollector(capturer Capturer, interval time.Duration, params map[string]string, target *os.File,
	logger golog.Logger) Collector {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	return &collector{
		queue:     make(chan *structpb.Struct, queueSize),
		interval:  interval,
		params:    params,
		lock:      &sync.Mutex{},
		logger:    logger,
		target:    target,
		writer:    bufio.NewWriter(target),
		cancelCtx: cancelCtx,
		cancel:    cancelFunc,
		capturer:  capturer,
	}
}

// TODO: length prefix when writing.
func (c *collector) write() error {
	for msg := range c.queue {
		if err := c.appendMessage(msg); err != nil {
			return err
		}
	}
	return nil
}

func (c *collector) appendMessage(msg *structpb.Struct) error {
	bytes, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	_, err = c.writer.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}

// InterfaceToStruct converts an arbitrary Go struct to a *structpb.Struct. Only exported fields are included in the
// returned proto.
func InterfaceToStruct(i interface{}) (*structpb.Struct, error) {
	encoded, err := protoutils.InterfaceToMap(i)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to convert interface %v to a form acceptable to structpb.NewStruct", i)
	}
	ret, err := structpb.NewStruct(encoded)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to construct structpb.Struct from map %v", encoded)

	}
	return ret, nil
}

// InvalidInterfaceErr is the error describing when an interface not conforming to the expected resource.Subtype was
// passed into a CollectorConstructor.
func InvalidInterfaceErr(typeName resource.SubtypeName) error {
	return errors.Errorf("passed interface does not conform to expected resource type %s", typeName)
}

// FailedToReadErr is the error describing when a Capturer was unable to get the reading of a method.
func FailedToReadErr(component string, method string) error {
	return errors.Errorf("failed to get reading of method %s of component %s", method, component)
}
