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
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/structpb"
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
	queue     chan *v1.SensorData
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

	c.cancel()
	if err := c.writer.Flush(); err != nil {
		c.logger.Errorw("failed to flush writer to disk", "error", err)
	}
}

// Collect starts the Collector, causing it to run c.capture every c.interval, and write the results to c.target.
func (c *collector) Collect() error {
	_, span := trace.StartSpan(c.cancelCtx, "data::collector::Collect")
	defer span.End()

	errs, _ := errgroup.WithContext(c.cancelCtx)
	go c.capture()
	errs.Go(func() error {
		return c.write()
	})
	return errs.Wait()
}

func (c *collector) capture() {
	var wg sync.WaitGroup
	lastTick := time.Now()
	next := lastTick.Add(c.interval)
	for {
		time.Sleep(time.Until(next))
		select {
		case <-c.cancelCtx.Done():
			wg.Wait()
			close(c.queue)
			return
		default:
			currTick := time.Now()
			diff := currTick.Sub(lastTick).Microseconds()
			if diff > 1800 {
				c.logger.Debugf("took %d microseconds between ticks", diff)
			}
			lastTick = currTick
			wg.Add(1)
			go c.getAndPushNextReading(&wg)
		}
		next = next.Add(c.interval)
	}
}

func (c *collector) getAndPushNextReading(wg *sync.WaitGroup) {
	defer wg.Done()

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
	pbReading, err := StructToStructPb(reading)
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
func NewCollector(capturer Capturer, interval time.Duration, params map[string]string, target *os.File, queueSize int,
	bufferSize int, logger golog.Logger) Collector {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	return &collector{
		queue:     make(chan *v1.SensorData, queueSize),
		interval:  interval,
		params:    params,
		lock:      &sync.Mutex{},
		logger:    logger,
		target:    target,
		writer:    bufio.NewWriterSize(target, bufferSize),
		cancelCtx: cancelCtx,
		cancel:    cancelFunc,
		capturer:  capturer,
	}
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

// StructToStructPb converts an arbitrary Go struct to a *structpb.Struct. Only exported fields are included in the
// returned proto.
func StructToStructPb(i interface{}) (*structpb.Struct, error) {
	encoded, err := protoutils.InterfaceToMap(i)
	if err != nil {
		return nil, errors.Errorf("unable to convert interface %v to a form acceptable to structpb.NewStruct: %v", i, err)
	}
	ret, err := structpb.NewStruct(encoded)
	if err != nil {
		return nil, errors.Errorf("unable to construct structpb.Struct from map %v: %v", encoded, err)
	}
	return ret, nil
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
