package data

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Capturer provides a function for capturing a single protobuf reading from the underlying component.
type Capturer interface {
	Capture(params map[string]string) (*any.Any, error)
}

// A Collector calls capturer at the specified Interval, and appends the resulting reading to target.
type Collector struct {
	queue    chan *any.Any
	Interval time.Duration
	params   map[string]string
	lock     *sync.Mutex
	target   *os.File
	done     chan bool
	capturer Capturer
}

// SetTarget updates the file being written to by the collector.
func (c *Collector) SetTarget(file *os.File) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.target = file
}

// GetTarget returns the file being written to by the collector.
func (c *Collector) GetTarget() *os.File {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.target
}

// Close closes the channels backing the Collector. It should always be called before disposing of a Collector to avoid
// leaking goroutines.
func (c *Collector) Close() {
	c.done <- true
	close(c.queue)
	close(c.done)
}

func (c *Collector) Collect(ctx context.Context) error {
	errs, ctx := errgroup.WithContext(ctx)
	errs.Go(c.capture)
	errs.Go(func() error {
		return c.write()
	})
	return errs.Wait()
}

func (c *Collector) capture() error {
	ticker := time.NewTicker(c.Interval)
	for {
		select {
		case <-c.done:
			return nil
		case <-ticker.C:
			a, err := c.capturer.Capture(c.params)
			if err != nil {
				return err
			}
			c.queue <- a
		}
	}
}

func NewCollector(capturer Capturer, interval time.Duration, params map[string]string, target *os.File) Collector {
	return Collector{
		queue:    make(chan *any.Any, 10),
		Interval: interval,
		params:   params,
		lock:     &sync.Mutex{},
		target:   target,
		done:     make(chan bool),
		capturer: capturer,
	}
}

// TODO: length prefix when writing.
func (c *Collector) write() error {
	for a := range c.queue {
		bytes, err := proto.Marshal(a)
		if err != nil {
			return err
		}
		_, err = c.target.Write(bytes)
		if err != nil {
			return err
		}
	}
	return nil
}

// WrapProtoAll is a convenience function that takes the protobuf, error output of some protobuf method,
// wraps the protobuf in any.Any, and returns any error if one is encountered.
func WrapProtoAll(msg proto.Message, err error) (*any.Any, error) {
	if err != nil {
		return nil, err
	}
	a, err := anypb.New(msg)
	if err != nil {
		return nil, err
	}
	return a, nil
}
