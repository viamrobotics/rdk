// Package data contains the code involved with Viam's Data Management Platform for automatically collecting component
// readings from robots.
package data

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/utils"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "go.viam.com/rdk/proto/api/service/datamanager/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// Capturer provides a function for capturing a single protobuf reading from the underlying component.
type Capturer interface {
	Capture(ctx context.Context, params map[string]string) (interface{}, error)
}

// CaptureFunc allows the creation of simple Capturers with anonymous functions.
type CaptureFunc func(ctx context.Context, params map[string]string) (interface{}, error)

// Capture allows any CaptureFunc to conform to the Capturer interface.
func (cf CaptureFunc) Capture(ctx context.Context, params map[string]string) (interface{}, error) {
	return cf(ctx, params)
}

// Collector collects data to some target.
type Collector interface {
	SetTarget(file *os.File)
	GetTarget() *os.File
	Close()
	Collect() error
}

type collector struct {
	queue             chan *v1.SensorData
	interval          time.Duration
	params            map[string]string
	lock              *sync.Mutex
	logger            golog.Logger
	target            *os.File
	writer            *bufio.Writer
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancel            context.CancelFunc
	capturer          Capturer
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

	c.cancel()
	c.backgroundWorkers.Wait()
	if err := c.writer.Flush(); err != nil {
		c.logger.Errorw("failed to flush writer to disk", "error", err)
	}
}

// Collect starts the Collector, causing it to run c.capturer.Capture every c.interval, and write the results to
// c.target.
func (c *collector) Collect() error {
	_, span := trace.StartSpan(c.cancelCtx, "data::collector::Collect")
	defer span.End()

	c.backgroundWorkers.Add(1)
	utils.PanicCapturingGo(c.capture)
	return c.write()
}

// Go's time.Ticker has inconsistent performance with durations of below 1ms [0], so we use a time.Sleep based approach
// when the configured capture interval is below 2ms. A Ticker based approach is kept for longer capture intervals to
// avoid wasting CPU on a thread that's idling for the vast majority of the time.
// [0]: https://www.mail-archive.com/golang-nuts@googlegroups.com/msg46002.html
func (c *collector) capture() {
	defer c.backgroundWorkers.Done()

	if c.interval < time.Millisecond*2 {
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
	pbReading, err := protoutils.StructToStructPb(reading)
	if err != nil {
		c.logger.Errorw("error while converting reading to structpb.Struct", "error", err)
		return
	}
	msg := v1.SensorData{
		Metadata: &v1.SensorMetadata{
			TimeRequested: timeRequested,
			TimeReceived:  timeReceived,
		},
		Data: pbReading,
	}

	select {
	// If c.qgitueue is full, c.queue <- a can block indefinitely. This additional select block allows cancel to
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
		target:            params.Target,
		writer:            bufio.NewWriterSize(params.Target, params.BufferSize),
		cancelCtx:         cancelCtx,
		cancel:            cancelFunc,
		backgroundWorkers: sync.WaitGroup{},
		capturer:          capturer,
	}, nil
}

func (c *collector) write() error {
	for msg := range c.queue {
		if err := c.appendMessage(msg); err != nil {
			return err
		}
	}
	return nil
}

func (c *collector) appendMessage(msg *v1.SensorData) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := pbutil.WriteDelimited(c.writer, msg)
	if err != nil {
		return err
	}
	return nil
}

// InvalidInterfaceErr is the error describing when an interface not conforming to the expected resource.Subtype was
// passed into a CollectorConstructor.
func InvalidInterfaceErr(typeName resource.SubtypeName) error {
	return errors.Errorf("passed interface does not conform to expected resource type %s", typeName)
}

// FailedToReadErr is the error describing when a Capturer was unable to get the reading of a method.
func FailedToReadErr(component string, method string, err error) error {
	return errors.Errorf("failed to get reading of method %s of component %s: %v", method, component, err)
}
