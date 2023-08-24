// Package data contains the code involved with Viam's Data Management Platform for automatically collecting component
// readings from robots.
package data

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager/datacapture"
)

// The cutoff at which if interval < cutoff, a sleep based capture func is used instead of a ticker.
var sleepCaptureCutoff = 2 * time.Millisecond

// CaptureFunc allows the creation of simple Capturers with anonymous functions.
type CaptureFunc func(ctx context.Context, params map[string]*anypb.Any) (interface{}, error)

// FromDMContextKey is used to check whether the context is from data management.
type FromDMContextKey struct{}

// FromDMString is used to access the 'fromDataManagement' value from a request's Extra struct.
const FromDMString = "fromDataManagement"

// ErrNoCaptureToStore is returned when a modular filter resource filters the capture coming from the base resource.
var ErrNoCaptureToStore = status.Error(codes.FailedPrecondition, "no capture from filter module")

// Collector collects data to some target.
type Collector interface {
	Close()
	Collect()
	Flush()
}

type collector struct {
	clock          clock.Clock
	captureResults chan *v1.SensorData
	captureErrors  chan error
	interval       time.Duration
	params         map[string]*anypb.Any
	lock           sync.Mutex
	logger         golog.Logger
	captureWorkers sync.WaitGroup
	logRoutine     sync.WaitGroup
	cancelCtx      context.Context
	cancel         context.CancelFunc
	captureFunc    CaptureFunc
	closed         bool
	target         datacapture.BufferedWriter
}

// Close closes the channels backing the Collector. It should always be called before disposing of a Collector to avoid
// leaking goroutines.
func (c *collector) Close() {
	if c.closed {
		return
	}
	c.closed = true

	c.cancel()
	c.captureWorkers.Wait()
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.target.Flush(); err != nil {
		c.logger.Errorw("failed to flush capture data", "error", err)
	}
	close(c.captureErrors)
	c.logRoutine.Wait()
	//nolint:errcheck
	_ = c.logger.Sync()
}

func (c *collector) Flush() {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.target.Flush(); err != nil {
		c.logger.Errorw("failed to flush collector", "error", err)
	}
}

// Collect starts the Collector, causing it to run c.capturer.Capture every c.interval, and write the results to
// c.target. It blocks until the underlying capture goroutine starts.
func (c *collector) Collect() {
	_, span := trace.StartSpan(c.cancelCtx, "data::collector::Collect")
	defer span.End()

	started := make(chan struct{})
	c.captureWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer c.captureWorkers.Done()
		c.capture(started)
	})
	c.captureWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer c.captureWorkers.Done()
		if err := c.writeCaptureResults(); err != nil {
			c.captureErrors <- errors.Wrap(err, fmt.Sprintf("failed to write to collector %s", c.target.Path()))
		}
	})
	c.logRoutine.Add(1)
	utils.PanicCapturingGo(func() {
		defer c.logRoutine.Done()
		c.logCaptureErrs()
	})
	<-started
}

// Go's time.Ticker has inconsistent performance with durations of below 1ms [0], so we use a time.Sleep based approach
// when the configured capture interval is below 2ms. A Ticker based approach is kept for longer capture intervals to
// avoid wasting CPU on a thread that's idling for the vast majority of the time.
// [0]: https://www.mail-archive.com/golang-nuts@googlegroups.com/msg46002.html
func (c *collector) capture(started chan struct{}) {
	if c.interval < sleepCaptureCutoff {
		c.sleepBasedCapture(started)
	} else {
		c.tickerBasedCapture(started)
	}
}

func (c *collector) sleepBasedCapture(started chan struct{}) {
	next := c.clock.Now().Add(c.interval)
	until := c.clock.Until(next)
	var captureWorkers sync.WaitGroup

	close(started)
	for {
		if err := c.cancelCtx.Err(); err != nil {
			c.captureErrors <- errors.Wrap(err, "error in context")
			captureWorkers.Wait()
			close(c.captureResults)
			return
		}
		c.clock.Sleep(until)

		select {
		case <-c.cancelCtx.Done():
			captureWorkers.Wait()
			close(c.captureResults)
			return
		default:
			captureWorkers.Add(1)
			utils.PanicCapturingGo(func() {
				defer captureWorkers.Done()
				c.getAndPushNextReading()
			})
		}
		next = next.Add(c.interval)
		until = c.clock.Until(next)
	}
}

func (c *collector) tickerBasedCapture(started chan struct{}) {
	ticker := c.clock.Ticker(c.interval)
	defer ticker.Stop()
	var captureWorkers sync.WaitGroup

	close(started)
	for {
		if err := c.cancelCtx.Err(); err != nil {
			c.captureErrors <- errors.Wrap(err, "error in context")
			captureWorkers.Wait()
			close(c.captureResults)
			return
		}

		select {
		case <-c.cancelCtx.Done():
			captureWorkers.Wait()
			close(c.captureResults)
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
	timeRequested := timestamppb.New(c.clock.Now().UTC())
	reading, err := c.captureFunc(c.cancelCtx, c.params)
	timeReceived := timestamppb.New(c.clock.Now().UTC())
	if err != nil {
		if errors.Is(err, ErrNoCaptureToStore) {
			c.logger.Debugln("capture filtered out by modular resource")
			return
		}
		c.captureErrors <- errors.Wrap(err, "error while capturing data")
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
			c.captureErrors <- errors.Wrap(err, "error while converting reading to structpb.Struct")
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
	// If c.captureResults is full, c.captureResults <- a can block indefinitely. This additional select block allows cancel to
	// still work when this happens.
	case <-c.cancelCtx.Done():
	case c.captureResults <- &msg:
	}
}

// NewCollector returns a new Collector with the passed capturer and configuration options. It calls capturer at the
// specified Interval, and appends the resulting reading to target.
func NewCollector(captureFunc CaptureFunc, params CollectorParams) (Collector, error) {
	if err := params.Validate(); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to construct collector for %s", params.ComponentName))
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	var c clock.Clock
	if params.Clock == nil {
		c = clock.New()
	} else {
		c = params.Clock
	}
	return &collector{
		captureResults: make(chan *v1.SensorData, params.QueueSize),
		captureErrors:  make(chan error, params.QueueSize),
		interval:       params.Interval,
		params:         params.MethodParams,
		logger:         params.Logger,
		cancelCtx:      cancelCtx,
		cancel:         cancelFunc,
		captureFunc:    captureFunc,
		target:         params.Target,
		clock:          c,
		closed:         false,
	}, nil
}

func (c *collector) writeCaptureResults() error {
	for msg := range c.captureResults {
		if err := c.target.Write(msg); err != nil {
			return err
		}
	}
	return nil
}

func (c *collector) logCaptureErrs() {
	for err := range c.captureErrors {
		if c.closed {
			// Don't log context cancellation errors if the collector has already been closed. This means the collector
			// cancelled the context, and the context cancellation error is expected.
			if errors.Is(err, context.Canceled) {
				continue
			}
		}
		c.logger.Error(err)
	}
}

// InvalidInterfaceErr is the error describing when an interface not conforming to the expected resource.API was
// passed into a CollectorConstructor.
func InvalidInterfaceErr(api resource.API) error {
	return errors.Errorf("passed interface does not conform to expected resource type %s", api)
}

// FailedToReadErr is the error describing when a Capturer was unable to get the reading of a method.
func FailedToReadErr(component, method string, err error) error {
	return errors.Errorf("failed to get reading of method %s of component %s: %v", method, component, err)
}

// GetExtraFromContext adds "fromDataManagement": true to the extra map if the flag is true in the context, and returns a protobuf Struct.
func GetExtraFromContext(ctx context.Context, extra map[string]interface{}) (*structpb.Struct, error) {
	if ctx.Value(FromDMContextKey{}) == true {
		extra[FromDMString] = true
	}
	return protoutils.StructToStructPb(extra)
}
