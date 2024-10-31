// Package data contains the code involved with Viam's Data Management Platform for automatically collecting component
// readings from robots.
package data

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opencensus.io/trace"
	dataPB "go.viam.com/api/app/data/v1"
	datasyncPB "go.viam.com/api/app/datasync/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// The cutoff at which if interval < cutoff, a sleep based capture func is used instead of a ticker.
var sleepCaptureCutoff = 2 * time.Millisecond

// CaptureFunc allows the creation of simple Capturers with anonymous functions.
type CaptureFunc func(ctx context.Context, params map[string]*anypb.Any) (CaptureResult, error)

// Timestamps are the timestamps the data was captured.
type Timestamps struct {
	// TimeRequested represents the time the request for the data was started
	TimeRequested time.Time
	// TimeRequested represents the time the response for the request for the data
	// was received
	TimeReceived time.Time
}

// MimeType represents the mime type of the sensor data.
type MimeType int

// This follows the mime types supported in
// https://github.com/viamrobotics/api/pull/571/files#diff-b77927298d8d5d5228beeea47bd0860d9b322b4f3ef45e129bc238ec17704826R75
const (
	// MimeTypeUnspecified means that the mime type was not specified.
	MimeTypeUnspecified MimeType = iota
	// MimeTypeImageJpeg means that the mime type is jpeg.
	MimeTypeImageJpeg
	// MimeTypeImagePng means that the mime type is png.
	MimeTypeImagePng
	// MimeTypeApplicationPcd means that the mime type is pcd.
	MimeTypeApplicationPcd
)

// CaptureResult is the result of a capture function.
type CaptureResult struct {
	Timestamps
	// Type represents the type of result (binary or tabular)
	Type CaptureType
	// TabularData contains the tabular data payload when Type == CaptureResultTypeTabular
	TabularData TabularData
	// Binaries contains binary data responses when Type == CaptureResultTypeBinary
	Binaries []Binary
}

// Binary represents an element of a binary capture result response.
type Binary struct {
	// Payload contains the binary payload
	Payload []byte
	// MimeType  descibes the payload's MimeType
	MimeType MimeType
	// Annotations provide metadata about the Payload
	Annotations Annotations
}

// ToProto converts MimeType to datasyncPB.
func (mt MimeType) ToProto() datasyncPB.MimeType {
	switch mt {
	case MimeTypeUnspecified:
		return datasyncPB.MimeType_MIME_TYPE_UNSPECIFIED
	case MimeTypeImageJpeg:
		return datasyncPB.MimeType_MIME_TYPE_IMAGE_JPEG
	case MimeTypeImagePng:
		return datasyncPB.MimeType_MIME_TYPE_IMAGE_PNG
	case MimeTypeApplicationPcd:
		return datasyncPB.MimeType_MIME_TYPE_APPLICATION_PCD
	default:
		return datasyncPB.MimeType_MIME_TYPE_UNSPECIFIED
	}
}

// TabularData contains a tabular data payload.
type TabularData struct {
	Payload *structpb.Struct
}

// BoundingBox represents a labeled bounding box
// with an optional confidence interval between 0 and 1.
type BoundingBox struct {
	Label          string
	Confidence     *float64
	XMinNormalized float64
	YMinNormalized float64
	XMaxNormalized float64
	YMaxNormalized float64
}

// Classification represents a labeled classification
// with an optional confidence interval between 0 and 1.
type Classification struct {
	Label      string
	Confidence *float64
}

// Annotations represents ML classifications.
type Annotations struct {
	BoundingBoxes   []BoundingBox
	Classifications []Classification
}

// ToProto converts Annotations to *dataPB.Annotations.
func (mt Annotations) ToProto() *dataPB.Annotations {
	var bboxes []*dataPB.BoundingBox
	for _, bb := range mt.BoundingBoxes {
		bboxes = append(bboxes, &dataPB.BoundingBox{
			Label:          bb.Label,
			Confidence:     bb.Confidence,
			XMinNormalized: bb.XMinNormalized,
			XMaxNormalized: bb.XMaxNormalized,
			YMinNormalized: bb.YMinNormalized,
			YMaxNormalized: bb.YMaxNormalized,
		})
	}

	var classifications []*dataPB.Classification
	for _, c := range mt.Classifications {
		classifications = append(classifications, &dataPB.Classification{
			Label:      c.Label,
			Confidence: c.Confidence,
		})
	}

	return &dataPB.Annotations{
		Bboxes:          bboxes,
		Classifications: classifications,
	}
}

// Validate returns an error if the *CaptureResult is invalid.
func (cr *CaptureResult) Validate() error {
	var ts Timestamps
	if cr.Timestamps.TimeRequested == ts.TimeRequested {
		return errors.New("Timestamps.TimeRequested must be set")
	}

	if cr.Timestamps.TimeReceived == ts.TimeReceived {
		return errors.New("Timestamps.TimeRequested must be set")
	}

	switch cr.Type {
	case CaptureTypeTabular:
		if len(cr.Binaries) > 0 {
			return errors.New("tabular result can't contain binary data")
		}
		if cr.TabularData.Payload == nil {
			return errors.New("tabular result must have non empty tabular data")
		}
		return nil
	case CaptureTypeBinary:
		if cr.TabularData.Payload != nil {
			return errors.New("binary result can't contain tabular data")
		}
		if len(cr.Binaries) == 0 {
			return errors.New("binary result must have non empty binary data")
		}

		for _, b := range cr.Binaries {
			if len(b.Payload) == 0 {
				return errors.New("binary results can't have empty binary payload")
			}
		}
		return nil
	case CaptureTypeUnspecified:
		return fmt.Errorf("unknown CaptureResultType: %d", cr.Type)
	default:
		return fmt.Errorf("unknown CaptureResultType: %d", cr.Type)
	}
}

// NewBinaryCaptureResult returns a binary capture result.
func NewBinaryCaptureResult(ts Timestamps, binaries []Binary) CaptureResult {
	return CaptureResult{
		Timestamps: ts,
		Type:       CaptureTypeBinary,
		Binaries:   binaries,
	}
}

// NewTabularCaptureResultReadings returns a tabular readings result.
func NewTabularCaptureResultReadings(reqT time.Time, readings map[string]interface{}) (CaptureResult, error) {
	var res CaptureResult
	values, err := rprotoutils.ReadingGoToProto(readings)
	if err != nil {
		return res, err
	}

	return CaptureResult{
		Timestamps: Timestamps{
			TimeRequested: reqT,
			TimeReceived:  time.Now(),
		},
		Type: CaptureTypeTabular,
		TabularData: TabularData{
			Payload: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"readings": structpb.NewStructValue(&structpb.Struct{Fields: values}),
				},
			},
		},
	}, nil
}

// NewTabularCaptureResult returns a tabular result.
func NewTabularCaptureResult(reqT time.Time, i interface{}) (CaptureResult, error) {
	var res CaptureResult
	readings, err := protoutils.StructToStructPbIgnoreOmitEmpty(i)
	if err != nil {
		return res, err
	}

	return CaptureResult{
		Timestamps: Timestamps{
			TimeRequested: reqT,
			TimeReceived:  time.Now(),
		},
		Type: CaptureTypeTabular,
		TabularData: TabularData{
			Payload: readings,
		},
	}, nil
}

// ToProto converts a CaptureResult into a []*datasyncPB.SensorData{}.
func (cr CaptureResult) ToProto() []*datasyncPB.SensorData {
	ts := cr.Timestamps
	if td := cr.TabularData.Payload; td != nil {
		return []*datasyncPB.SensorData{{
			Metadata: &datasyncPB.SensorMetadata{
				TimeRequested: timestamppb.New(ts.TimeRequested.UTC()),
				TimeReceived:  timestamppb.New(ts.TimeReceived.UTC()),
			},
			Data: &datasyncPB.SensorData_Struct{
				Struct: td,
			},
		}}
	}

	var sd []*datasyncPB.SensorData
	for _, b := range cr.Binaries {
		sd = append(sd, &datasyncPB.SensorData{
			Metadata: &datasyncPB.SensorMetadata{
				TimeRequested: timestamppb.New(ts.TimeRequested.UTC()),
				TimeReceived:  timestamppb.New(ts.TimeReceived.UTC()),
				MimeType:      b.MimeType.ToProto(),
				Annotations:   b.Annotations.ToProto(),
			},
			Data: &datasyncPB.SensorData_Binary{
				Binary: b.Payload,
			},
		})
	}
	return sd
}

// CameraFormatToMimeType converts a camera camerapb.Format into a MimeType.
func CameraFormatToMimeType(f camerapb.Format) MimeType {
	if f == camerapb.Format_FORMAT_JPEG {
		return MimeTypeImageJpeg
	}

	if f == camerapb.Format_FORMAT_PNG {
		return MimeTypeImagePng
	}
	return MimeTypeUnspecified
}

// FromDMContextKey is used to check whether the context is from data management.
// Deprecated: use a camera.Extra with camera.NewContext instead.
type FromDMContextKey struct{}

// FromDMString is used to access the 'fromDataManagement' value from a request's Extra struct.
const FromDMString = "fromDataManagement"

// FromDMExtraMap is a map with 'fromDataManagement' set to true.
var FromDMExtraMap = map[string]interface{}{FromDMString: true}

// ErrNoCaptureToStore is returned when a modular filter resource filters the capture coming from the base resource.
var ErrNoCaptureToStore = status.Error(codes.FailedPrecondition, "no capture from filter module")

// If an error is ongoing, the frequency (in seconds) with which to suppress identical error logs.
const identicalErrorLogFrequencyHz = 2

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
	reading, err := c.captureFunc(c.cancelCtx, c.params)

	if c.cancelCtx.Err() != nil {
		return
	}

	if err != nil {
		if errors.Is(err, ErrNoCaptureToStore) {
			c.logger.Debug("capture filtered out by modular resource")
			return
		}
		c.captureErrors <- errors.Wrap(err, "error while capturing data")
		return
	}

	if err := c.validateReadingType(reading.Type); err != nil {
		c.captureErrors <- errors.Wrap(err, "capture result invalid type")
		return
	}

	if err := reading.Validate(); err != nil {
		c.captureErrors <- errors.Wrap(err, "capture result failed validation")
		return
	}

	select {
	// If c.captureResults is full, c.captureResults <- a can block indefinitely.
	// This additional select block allows cancel to
	// still work when this happens.
	case <-c.cancelCtx.Done():
	case c.captureResults <- reading:
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
				if err := c.target.WriteTabular(proto); err != nil {
					c.logger.Error(errors.Wrap(err, fmt.Sprintf("failed to write tabular data to prog file %s", c.target.Path())).Error())
					return
				}
			case CaptureTypeBinary:
				if err := c.target.WriteBinary(proto); err != nil {
					c.logger.Error(errors.Wrap(err, fmt.Sprintf("failed to write binary data to prog file %s", c.target.Path())).Error())
					return
				}
			case CaptureTypeUnspecified:
				c.logger.Error(fmt.Sprintf("collector returned invalid result type: %d", msg.Type))
				return
			default:
				c.logger.Error(fmt.Sprintf("collector returned invalid result type: %d", msg.Type))
				return
			}

			c.maybeWriteToMongo(msg)
		}
	}
}

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
			c.logger.Error((err))
			c.lastLoggedErrors[err.Error()] = now
		}
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

// GetExtraFromContext sets the extra struct with "fromDataManagement": true if the flag is true in the context.
// Deprecated: Use camera.FromContext instead.
func GetExtraFromContext(ctx context.Context) (*structpb.Struct, error) {
	extra := make(map[string]interface{})
	if ctx.Value(FromDMContextKey{}) == true {
		extra[FromDMString] = true
	}
	return protoutils.StructToStructPb(extra)
}
