// Package data contains the code involved with Viam's Data Management Platform for automatically collecting component
// readings from robots.
package data

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/api/app/datasync/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager/datacapture"
)

// The cutoff at which if interval < cutoff, a sleep based capture func is used instead of a ticker.
var sleepCaptureCutoff = 2 * time.Millisecond

// Capturer provides a function for capturing a single protobuf reading from the underlying component.
type Capturer interface {
	Capture(ctx context.Context, params map[string]*anypb.Any) (interface{}, error)
}

// CaptureFunc allows the creation of simple Capturers with anonymous functions.
type CaptureFunc func(ctx context.Context, params map[string]*anypb.Any) (interface{}, error)

// Capture allows any CaptureFunc to conform to the Capturer interface.
func (cf CaptureFunc) Capture(ctx context.Context, params map[string]*anypb.Any) (interface{}, error) {
	return cf(ctx, params)
}

// Collector collects data to some target.
type Collector interface {
	Close()
	Collect()
}

type collector struct {
	queue             chan *v1.SensorData
	interval          time.Duration
	params            map[string]*anypb.Any
	lock              *sync.Mutex
	logger            golog.Logger
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancel            context.CancelFunc
	capturer          Capturer
	closed            bool

	target *datacapture.Buffer
}

// Close closes the channels backing the Collector. It should always be called before disposing of a Collector to avoid
// leaking goroutines.
func (c *collector) Close() {
	if c.closed {
		return
	}
	c.cancel()
	c.backgroundWorkers.Wait()
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.target.Flush(); err != nil {
		c.logger.Errorw("failed to sync capture queue", "error", err)
	}
	c.closed = true
}

// Collect starts the Collector, causing it to run c.capturer.Capture every c.interval, and write the results to
// c.target.
func (c *collector) Collect() {
	_, span := trace.StartSpan(c.cancelCtx, "data::collector::Collect")
	defer span.End()

	c.backgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer c.backgroundWorkers.Done()
		c.capture()
	})
	c.backgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer c.backgroundWorkers.Done()
		if err := c.write(); err != nil {
			c.logger.Errorw(fmt.Sprintf("failed to write to collector %s", c.target.Directory), "error", err)
		}
	})
}

// Go's time.Ticker has inconsistent performance with durations of below 1ms [0], so we use a time.Sleep based approach
// when the configured capture interval is below 2ms. A Ticker based approach is kept for longer capture intervals to
// avoid wasting CPU on a thread that's idling for the vast majority of the time.
// [0]: https://www.mail-archive.com/golang-nuts@googlegroups.com/msg46002.html
func (c *collector) capture() {
	if c.interval < sleepCaptureCutoff {
		c.sleepBasedCapture()
	} else {
		c.tickerBasedCapture()
	}
}

func (c *collector) sleepBasedCapture() {
	next := time.Now().Add(c.interval)
	captureWorkers := sync.WaitGroup{}

	for {
		time.Sleep(time.Until(next))
		if err := c.cancelCtx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				c.logger.Errorw("unexpected error in collector context", "error", err)
			}
			captureWorkers.Wait()
			close(c.queue)
			return
		}

		select {
		case <-c.cancelCtx.Done():
			captureWorkers.Wait()
			close(c.queue)
			return
		default:
			captureWorkers.Add(1)
			utils.PanicCapturingGo(func() {
				defer captureWorkers.Done()
				c.getAndPushNextReading()
			})
		}
		next = next.Add(c.interval)
	}
}

func (c *collector) tickerBasedCapture() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	captureWorkers := sync.WaitGroup{}

	for {
		if err := c.cancelCtx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				c.logger.Errorw("unexpected error in collector context", "error", err)
			}
			captureWorkers.Wait()
			close(c.queue)
			return
		}

		select {
		case <-c.cancelCtx.Done():
			captureWorkers.Wait()
			close(c.queue)
			return
		case <-ticker.C:
			captureWorkers.Add(1)
			utils.PanicCapturingGo(func() {
				defer captureWorkers.Done()
				c.getAndPushNextReading()
			})
		}
	}
}

func (c *collector) getAndPushNextReading() {
	timeRequested := timestamppb.New(time.Now().UTC())
	reading, err := c.capturer.Capture(c.cancelCtx, c.params)
	timeReceived := timestamppb.New(time.Now().UTC())
	if err != nil {
		if errors.Is(err, context.Canceled) {
			c.logger.Debugw("error while capturing data", "error", err)
			return
		}
		c.logger.Errorw("error while capturing data", "error", err)
		return
	}

	var msg v1.SensorData
	switch v := reading.(type) {
	case []byte:
		msg = v1.SensorData{
			Metadata: &v1.SensorMetadata{
				TimeRequested: timeRequested,
				TimeReceived:  timeReceived,
			},
			Data: &v1.SensorData_Binary{
				Binary: v,
			},
		}
	default:
		// If it's not bytes, it's a struct.
		pbReading, err := protoutils.StructToStructPb(reading)
		if err != nil {
			c.logger.Errorw("error while converting reading to structpb.Struct", "error", err)
			return
		}

		msg = v1.SensorData{
			Metadata: &v1.SensorMetadata{
				TimeRequested: timeRequested,
				TimeReceived:  timeReceived,
			},
			Data: &v1.SensorData_Struct{
				Struct: pbReading,
			},
		}
	}

	select {
	// If c.queue is full, c.queue <- a can block indefinitely. This additional select block allows cancel to
	// still work when this happens.
	case <-c.cancelCtx.Done():
		return
	case c.queue <- &msg:
		return
	}
}

// NewCollector returns a new Collector with the passed capturer and configuration options. It calls capturer at the
// specified Interval, and appends the resulting reading to target.
func NewCollector(capturer Capturer, params CollectorParams) (Collector, error) {
	if err := params.Validate(); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to construct collector for %s", params.ComponentName))
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	return &collector{
		queue:             make(chan *v1.SensorData, params.QueueSize),
		interval:          params.Interval,
		params:            params.MethodParams,
		lock:              &sync.Mutex{},
		logger:            params.Logger,
		backgroundWorkers: sync.WaitGroup{},
		cancelCtx:         cancelCtx,
		cancel:            cancelFunc,
		capturer:          capturer,
		target:            params.Target,
	}, nil
}

func (c *collector) write() error {
	for msg := range c.queue {
		if err := c.target.Write(msg); err != nil {
			return err
		}
	}
	return nil
}

// InvalidInterfaceErr is the error describing when an interface not conforming to the expected resource.Subtype was
// passed into a CollectorConstructor.
func InvalidInterfaceErr(typeName resource.SubtypeName) error {
	return errors.Errorf("passed interface does not conform to expected resource type %s", typeName)
}

// FailedToReadErr is the error describing when a Capturer was unable to get the reading of a method.
func FailedToReadErr(component, method string, err error) error {
	return errors.Errorf("failed to get reading of method %s of component %s: %v", method, component, err)
}
