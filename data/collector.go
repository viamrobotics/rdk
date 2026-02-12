// Package data contains the code involved with Viam's Data Management Platform for automatically collecting component
// readings from robots.
package data

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.viam.com/utils"
	"go.viam.com/utils/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// The cutoff at which if interval < cutoff, a sleep based capture func is used instead of a ticker.
var sleepCaptureCutoff = 2 * time.Millisecond

// FromDMString is used to access the 'fromDataManagement' value from a request's Extra struct.
const FromDMString = "fromDataManagement"

// FromDMExtraMap is a map with 'fromDataManagement' set to true.
var FromDMExtraMap = map[string]interface{}{FromDMString: true}

// ErrNoCaptureToStore is returned when a modular filter resource filters the capture coming from the base resource.
var (
	errNoCaptureToStoreMsg = "no capture from filter module"
	ErrNoCaptureToStore    = status.Error(codes.FailedPrecondition, errNoCaptureToStoreMsg)
)

// If an error is ongoing, the frequency (in seconds) with which to suppress identical error logs.
const identicalErrorLogFrequencyHz = 2

// TabularDataBson is a denormalized sensor reading that can be
// encoded into BSON.
type TabularDataBson struct {
	TimeRequested time.Time `bson:"time_requested"`
	TimeReceived  time.Time `bson:"time_received"`
	ComponentName string    `bson:"component_name"`
	ComponentType string    `bson:"component_type"`
	MethodName    string    `bson:"method_name"`
	Data          bson.M    `bson:"data"`
}

// Collector collects data to some target.
type Collector interface {
	Close()
	Collect()
	Flush()
}

type collector struct {
	clock clock.Clock

	captureResults  chan CaptureResult
	mongoCollection *mongo.Collection
	componentName   string
	componentType   string
	methodName      string
	captureErrors   chan error
	interval        time.Duration
	params          map[string]*anypb.Any
	// `lock` serializes calls to `Flush` and `Close`.
	lock             sync.Mutex
	logger           logging.Logger
	captureWorkers   sync.WaitGroup
	logRoutine       sync.WaitGroup
	cancelCtx        context.Context
	cancel           context.CancelFunc
	captureFunc      CaptureFunc
	target           CaptureBufferedWriter
	lastLoggedErrors map[string]int64
	dataType         CaptureType
}

// Close closes the channels backing the Collector. It should always be called before disposing of a Collector to avoid
// leaking goroutines.
func (c *collector) Close() {
	if c.cancelCtx.Err() != nil {
		return
	}

	// Signal all `captureWorkers` to exit.
	c.cancel()
	// CaptureWorkers acquire the `c.lock` to do their work (i.e: call `collector.Flush()`). We must
	// `Wait` on them before acquiring the lock to avoid deadlock.
	c.captureWorkers.Wait()

	c.Flush()

	close(c.captureErrors)
	c.logRoutine.Wait()
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
	utils.ManagedGo(func() { c.capture(started) }, c.captureWorkers.Done)
	c.captureWorkers.Add(1)
	utils.ManagedGo(c.writeCaptureResults, c.captureWorkers.Done)
	c.logRoutine.Add(1)
	utils.ManagedGo(c.logCaptureErrs, c.logRoutine.Done)

	// We must wait on `started` before returning. The sleep/ticker based captures rely on the clock
	// advancing to do their first "tick". They must make an initial clock reading before unittests
	// add an "interval". Lest the ticker never fires and a reading is never made.
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

	close(started)
	for {
		if err := c.cancelCtx.Err(); err != nil {
			return
		}
		c.clock.Sleep(until)
		if err := c.cancelCtx.Err(); err != nil {
			return
		}

		c.getAndPushNextReading()
		next = next.Add(c.interval)
		until = c.clock.Until(next)
	}
}

func (c *collector) tickerBasedCapture(started chan struct{}) {
	ticker := c.clock.Ticker(c.interval)
	defer ticker.Stop()

	close(started)
	for {
		if err := c.cancelCtx.Err(); err != nil {
			return
		}

		select {
		case <-c.cancelCtx.Done():
			return
		case <-ticker.C:
			c.getAndPushNextReading()
		}
	}
}

func (c *collector) validateReadingType(t CaptureType) error {
	switch c.dataType {
	case CaptureTypeTabular:
		if t != CaptureTypeTabular {
			return fmt.Errorf("expected result of type CaptureTypeTabular, instead got CaptureResultType: %d", t)
		}
		return nil
	case CaptureTypeBinary:
		if t != CaptureTypeBinary {
			return fmt.Errorf("expected result of type CaptureTypeBinary,instead got CaptureResultType: %d", t)
		}
		return nil
	case CaptureTypeUnspecified:
		return fmt.Errorf("unknown collector data type: %d", c.dataType)
	default:
		return fmt.Errorf("unknown collector data type: %d", c.dataType)
	}
}

func (c *collector) getAndPushNextReading() {
	result, err := c.captureFunc(c.cancelCtx, c.params)

	if c.cancelCtx.Err() != nil {
		return
	}

	if err != nil {
		if IsNoCaptureToStoreError(err) {
			c.logger.Debug("capture filtered out by modular resource")
			return
		}
		c.captureErrors <- errors.Wrap(err, "error while capturing data")
		return
	}

	if err := c.validateReadingType(result.Type); err != nil {
		c.captureErrors <- errors.Wrap(err, "capture result invalid type")
		return
	}

	if err := result.Validate(); err != nil {
		c.captureErrors <- errors.Wrap(err, "capture result failed validation")
		return
	}

	select {
	// If c.captureResults is full, c.captureResults <- a can block indefinitely.
	// This additional select block allows cancel to
	// still work when this happens.
	case <-c.cancelCtx.Done():
	case c.captureResults <- result:
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
		componentName:    params.ComponentName,
		componentType:    params.ComponentType,
		methodName:       params.MethodName,
		mongoCollection:  params.MongoCollection,
		captureResults:   make(chan CaptureResult, params.QueueSize),
		captureErrors:    make(chan error, params.QueueSize),
		dataType:         params.DataType,
		interval:         params.Interval,
		params:           params.MethodParams,
		logger:           params.Logger,
		cancelCtx:        cancelCtx,
		cancel:           cancelFunc,
		captureFunc:      captureFunc,
		target:           params.Target,
		clock:            c,
		lastLoggedErrors: make(map[string]int64, 0),
	}, nil
}

func (c *collector) writeCaptureResults() {
	for {
		if c.cancelCtx.Err() != nil {
			return
		}

		select {
		case <-c.cancelCtx.Done():
			return
		case msg := <-c.captureResults:
			proto := msg.ToProto()

			switch msg.Type {
			case CaptureTypeTabular:
				if len(proto) != 1 {
					// This is impossible and could only happen if a future code change breaks CaptureResult.ToProto()
					err := errors.New("tabular CaptureResult returned more than one tabular result")
					c.logger.Error(errors.Wrap(err, fmt.Sprintf("failed to write tabular data to prog file %s", c.target.Path())).Error())
					return
				}
				if err := c.target.WriteTabular(proto[0]); err != nil {
					c.logger.Error(errors.Wrap(err, fmt.Sprintf("failed to write tabular data to prog file %s", c.target.Path())).Error())
					return
				}
			case CaptureTypeBinary:
				if err := c.target.WriteBinary(proto); err != nil {
					c.logger.Error(errors.Wrap(err, fmt.Sprintf("failed to write binary data to prog file %s", c.target.Path())).Error())
					return
				}
			case CaptureTypeUnspecified:
				c.logger.Errorf("collector returned invalid result type: %d", msg.Type)
				return
			default:
				c.logger.Errorf("collector returned invalid result type: %d", msg.Type)
				return
			}

			c.maybeWriteToMongo(msg)
		}
	}
}

// maybeWriteToMongo will write to the mongoCollection
// if it is non-nil and the msg is tabular data
// logs errors on failure.
func (c *collector) maybeWriteToMongo(msg CaptureResult) {
	if c.mongoCollection == nil {
		return
	}

	if msg.Type != CaptureTypeTabular {
		return
	}

	s := msg.TabularData.Payload
	if s == nil {
		return
	}

	data, err := pbStructToBSON(s)
	if err != nil {
		c.logger.Error(errors.Wrap(err, "failed to convert sensor data into bson"))
		return
	}

	td := TabularDataBson{
		TimeRequested: msg.TimeRequested,
		TimeReceived:  msg.TimeReceived,
		ComponentName: c.componentName,
		ComponentType: c.componentType,
		MethodName:    c.methodName,
		Data:          data,
	}

	if _, err := c.mongoCollection.InsertOne(c.cancelCtx, td); err != nil {
		c.logger.Error(errors.Wrap(err, "failed to write to mongo"))
	}
}

func (c *collector) logCaptureErrs() {
	for err := range c.captureErrors {
		now := c.clock.Now().Unix()
		if c.cancelCtx.Err() != nil {
			// Don't log context cancellation errors if the collector has already been closed. This
			// means the collector canceled the context, and the context cancellation error is
			// expected.
			if errors.Is(err, context.Canceled) {
				continue
			}
		}

		// Only log a specific error message if we haven't logged it in the past 2 seconds.
		if lastLogged, ok := c.lastLoggedErrors[err.Error()]; (ok && int(now-lastLogged) > identicalErrorLogFrequencyHz) || !ok {
			var failedToReadError *FailedToReadError
			if errors.As(err, &failedToReadError) {
				c.logger.Warn(err)
			} else {
				c.logger.Error((err))
			}
			c.lastLoggedErrors[err.Error()] = now
		}
	}
}

// InvalidInterfaceErr is the error describing when an interface not conforming to the expected resource.API was
// passed into a CollectorConstructor.
func InvalidInterfaceErr(api resource.API) error {
	return errors.Errorf("passed interface does not conform to expected resource type %s", api)
}

// NewFailedToReadError constructs a new FailedToReadError.
func NewFailedToReadError(component, method string, err error) error {
	return &FailedToReadError{
		Component: component,
		Method:    method,
		Err:       err,
	}
}

// FailedToReadError is the error describing when a Capturer was unable to get the reading of a method.
type FailedToReadError struct {
	Component string
	Method    string
	Err       error
}

func (e *FailedToReadError) Error() string {
	return fmt.Sprintf("failed to get reading of method %s of component %s: %v", e.Method, e.Component, e.Err)
}

func (e *FailedToReadError) Unwrap() error {
	return e.Err
}

// NewDoCommandCaptureFunc returns a capture function for DoCommand operations that can be used by any resource.
// Components should assert their specific type and pass it to this function.
func NewDoCommandCaptureFunc[T interface {
	DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}](resource T, params CollectorParams) CaptureFunc {
	return func(ctx context.Context, _ map[string]*anypb.Any) (CaptureResult, error) {
		timeRequested := time.Now()
		var result CaptureResult

		var payload map[string]interface{}

		if payloadAny, exists := params.MethodParams["docommand_input"]; exists && payloadAny != nil {
			// Check if payloadAny is an empty map (empty *anypb.Any)
			if payloadAny.TypeUrl == "" && len(payloadAny.Value) == 0 {
				payload = make(map[string]interface{})
			} else {
				unmarshaledPayload, err := UnmarshalToValueOrString(payloadAny)
				if err != nil {
					return result, err
				}

				if payloadMap, ok := unmarshaledPayload.(map[string]interface{}); ok {
					payload = payloadMap
				} else {
					return result, fmt.Errorf("payload is not a map, got type: %T, value: %v", unmarshaledPayload, unmarshaledPayload)
				}
			}
		} else {
			// key does not exist
			return result, errors.New("DoCommand missing payload with key: \"docommand_input\"")
		}

		values, err := resource.DoCommand(ctx, payload)
		if err != nil {
			if IsNoCaptureToStoreError(err) {
				return result, err
			}
			return result, NewFailedToReadError(params.ComponentName, "DoCommand", err)
		}
		ts := Timestamps{TimeRequested: timeRequested, TimeReceived: time.Now()}
		return NewTabularCaptureResultDoCommand(ts, values)
	}
}

// UnmarshalToValueOrString attempts to unmarshal a protobuf Any to either a structpb.Value
// or extracts the string value if it's a string type.
func UnmarshalToValueOrString(v *anypb.Any) (interface{}, error) {
	// Try to unmarshal to Struct first
	structVal := &structpb.Struct{}
	if err := v.UnmarshalTo(structVal); err == nil {
		result := make(map[string]interface{})
		for fieldName, fieldValue := range structVal.Fields {
			result[fieldName] = flattenValue(fieldValue)
		}
		return result, nil
	}

	// Try to unmarshal to Value
	val := &structpb.Value{}
	if err := v.UnmarshalTo(val); err == nil {
		return flattenValue(val), nil
	}

	// If unmarshaling fails, try to unmarshal to string
	stringVal, err := v.UnmarshalNew()
	if err != nil {
		return nil, err
	}
	return stringVal, nil
}

// flattenValue extracts the actual value from a structpb.Value, removing protobuf metadata.
func flattenValue(val *structpb.Value) interface{} {
	switch v := val.Kind.(type) {
	case *structpb.Value_NullValue:
		return nil
	case *structpb.Value_NumberValue:
		return v.NumberValue
	case *structpb.Value_StringValue:
		return v.StringValue
	case *structpb.Value_BoolValue:
		return v.BoolValue
	case *structpb.Value_StructValue:
		// Flatten struct fields recursively
		result := make(map[string]interface{})
		for fieldName, fieldValue := range v.StructValue.Fields {
			result[fieldName] = flattenValue(fieldValue)
		}
		return result
	case *structpb.Value_ListValue:
		// Flatten list values recursively
		result := make([]interface{}, len(v.ListValue.Values))
		for i, item := range v.ListValue.Values {
			result[i] = flattenValue(item)
		}
		return result
	default:
		return val
	}
}

// IsNoCaptureToStoreError returns true if the error is NoCaptureToStoreError. Use this instead of errors.Is.
func IsNoCaptureToStoreError(err error) bool {
	return status.Code(err) == codes.FailedPrecondition && strings.Contains(err.Error(), errNoCaptureToStoreMsg)
}
