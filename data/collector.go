// Package data contains the code involved with Viam's Data Management Platform for automatically collecting component
// readings from robots.
package data

import (
	"bufio"
	"context"
	"os"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// queueSize defines the size of Collector's queue. It should be big enough to ensure that .capture() is never blocked
// by the queue being written to disk. A default value of 25 was chosen because even with the fastest reasonable capture
// interval (10ms), this would leave 250ms for a (buffered) disk write before blocking, which seems sufficient for the
// size of writes this would be performing.
const queueSize = 25

// Capturer provides a function for capturing a single protobuf reading from the underlying component.
type Capturer interface {
	Capture(params map[string]string) (*any.Any, error)
}

type Collector interface {
	SetTarget(file *os.File)
	GetTarget() *os.File
	Close()
	Collect(ctx context.Context) error
}

// A Collector calls capturer at the specified Interval, and appends the resulting reading to target.
type collector struct {
	queue    chan *any.Any
	interval time.Duration
	params   map[string]string
	lock     *sync.Mutex
	logger   golog.Logger
	target   *os.File
	writer   *bufio.Writer
	done     chan bool
	capturer Capturer
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
	c.done <- true
	close(c.queue)
	close(c.done)

	if err := c.writer.Flush(); err != nil {
		c.logger.Errorw("failed to flush writer to disk", "error", err)
	}
}

// TODO: Decide on error behavior here. Should receiving a single error from c.capture cause this to return an error?
//       I think the approach here will be more well informed when we start implementing the Data Manager Service and
//       actually using Collectors. I'm going to leave the behavior here for then. As is, I'll leave it just logging
//       errors.

// Collect starts the Collector, causing it to run c.capture every c.interval, and write the results to c.target.
func (c *collector) Collect(ctx context.Context) error {
	errs, _ := errgroup.WithContext(ctx)
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
		case <-c.done:
			return
		case <-ticker.C:
			a, err := c.capturer.Capture(c.params)
			if err != nil {
				c.logger.Errorw("error while capturing data", "error", err)
			}
			c.queue <- a
		}
	}
}

// NewCollector returns a new Collector with the passed capturer and configuration options.
func NewCollector(capturer Capturer, interval time.Duration, params map[string]string, target *os.File,
	logger golog.Logger) Collector {
	return &collector{
		queue:    make(chan *any.Any, queueSize),
		interval: interval,
		params:   params,
		lock:     &sync.Mutex{},
		logger:   logger,
		target:   target,
		writer:   bufio.NewWriter(target),
		done:     make(chan bool),
		capturer: capturer,
	}
}

// TODO: length prefix when writing.
func (c *collector) write() error {
	for a := range c.queue {
		if err := c.appendMessage(a); err != nil {
			return err
		}
	}
	return nil
}

func (c *collector) appendMessage(msg *any.Any) error {
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

// WrapInAll is a convenience function that takes the (proto.Message, error) output of some protobuf method,
// wraps the protobuf in any.Any, and returns any error if one is encountered.
func WrapInAll(msg proto.Message, err error) (*any.Any, error) {
	if err != nil {
		return nil, err
	}
	a, err := anypb.New(msg)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// MissingParameterErr returns an error with a mesage describing what parameter is missing for the given method.
func MissingParameterErr(param string, method string) error {
	return errors.Errorf("must pass parameter %s to method %s", param, method)
}
