package data

import (
	"context"
	"github.com/golang/protobuf/ptypes/any"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"os"
	"sync"
	"time"
)

// Capturer provides a function for capturing a single protobuf reading from the underlying component.
type Capturer interface {
	// TODO: generalize to arbitrary params
	Capture(name string) (*any.Any, error)
}

// A Collector calls capturer at the specified Interval, and appends the resulting reading to target.
type Collector struct {
	queue    chan *any.Any
	Interval time.Duration
	name     string
	lock     *sync.Mutex
	target   *os.File
	done     chan bool
	capturer Capturer
}

func (c *Collector) SetTarget(file *os.File) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.target = file
}

func (c *Collector) GetTarget() *os.File {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.target
}

func (c *Collector) Close() {
	c.done <- true
	close(c.queue)
	close(c.done)
}

func (c *Collector) Collect(ctx context.Context) error {
	errs, ctx := errgroup.WithContext(ctx)
	errs.Go(c.capture)
	errs.Go(func() error {
		return Write(c.queue, c.target)
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
			// TODO: generalize to arbitraru params
			a, err := c.capturer.Capture(c.name)
			if err != nil {
				return err
			}
			c.queue <- a
		}
	}
}

func NewCollector(capturer Capturer, interval time.Duration, name string, target *os.File) Collector {
	return Collector{
		queue:    make(chan *any.Any, 10),
		Interval: interval,
		name:     name,
		lock:     &sync.Mutex{},
		target:   target,
		done:     make(chan bool),
		capturer: capturer,
	}
}

// TODO: length prefix when writing
func Write(c chan *any.Any, target *os.File) error {
	for a := range c {
		bytes, err := proto.Marshal(a)
		if err != nil {
			return err
		}
		_, err = target.Write(bytes)
		if err != nil {
			return err
		}
	}
	return nil
}
