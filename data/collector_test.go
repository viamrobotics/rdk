package data

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/logging"
)

var (
	dummyTime      = time.Date(2024, time.January, 10, 23, 0, 0, 0, time.UTC)
	structCapturer = CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (CaptureResult, error) {
		return dummyStructReading, nil
	})
	binaryCapturer = CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (CaptureResult, error) {
		return CaptureResult{
			Timestamps: Timestamps{
				TimeRequested: dummyTime,
				TimeReceived:  dummyTime.Add(time.Second),
			},
			Type:     CaptureTypeBinary,
			Binaries: []Binary{{Payload: dummyBytesReading}},
		}, nil
	})
	dummyStructReading = CaptureResult{
		Timestamps: Timestamps{
			TimeRequested: dummyTime,
			TimeReceived:  dummyTime.Add(time.Second),
		},
		Type:        CaptureTypeTabular,
		TabularData: TabularData{dummyStructReadingProto},
	}
	dummyStructReadingProto = structReading{}.toProto()
	dummyBytesReading       = []byte("I sure am bytes")
	queueSize               = 250
	bufferSize              = 4096
	fakeVal                 = &anypb.Any{}
)

func TestNewCollector(t *testing.T) {
	// If missing parameters should return an error.
	c1, err1 := NewCollector(nil, CollectorParams{})

	test.That(t, c1, test.ShouldBeNil)
	test.That(t, err1, test.ShouldNotBeNil)

	// If not missing parameters, should not return an error.
	c2, err2 := NewCollector(nil, CollectorParams{
		DataType:      CaptureTypeTabular,
		ComponentName: "name",
		Logger:        logging.NewTestLogger(t),
		Target:        NewCaptureBuffer("dir", nil, 50),
	})

	test.That(t, err2, test.ShouldBeNil)
	test.That(t, c2, test.ShouldNotBeNil)

	c3, err3 := NewCollector(nil, CollectorParams{
		DataType:      CaptureTypeBinary,
		ComponentName: "name",
		Logger:        logging.NewTestLogger(t),
		Target:        NewCaptureBuffer("dir", nil, 50),
	})

	test.That(t, err3, test.ShouldBeNil)
	test.That(t, c3, test.ShouldNotBeNil)
}

// Test that the Collector correctly writes the SensorData on an interval.
func TestSuccessfulWrite(t *testing.T) {
	l := logging.NewTestLogger(t)
	tickerInterval := sleepCaptureCutoff + 1
	sleepInterval := sleepCaptureCutoff - 1

	params := CollectorParams{
		ComponentName: "testComponent",
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        l,
	}

	tests := []struct {
		name           string
		captureFunc    CaptureFunc
		interval       time.Duration
		expectReadings int
		expFiles       int
		datatype       CaptureType
	}{
		{
			name:           "Ticker based struct writer.",
			captureFunc:    structCapturer,
			interval:       tickerInterval,
			expectReadings: 2,
			expFiles:       1,
			datatype:       CaptureTypeTabular,
		},
		{
			name:           "Sleep based struct writer.",
			captureFunc:    structCapturer,
			interval:       sleepInterval,
			expectReadings: 2,
			expFiles:       1,
			datatype:       CaptureTypeTabular,
		},
		{
			name:           "Ticker based binary writer.",
			captureFunc:    binaryCapturer,
			interval:       tickerInterval,
			expectReadings: 2,
			expFiles:       2,
			datatype:       CaptureTypeBinary,
		},
		{
			name:           "Sleep based binary writer.",
			captureFunc:    binaryCapturer,
			interval:       sleepInterval,
			expectReadings: 2,
			expFiles:       2,
			datatype:       CaptureTypeBinary,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
			defer cancel()
			tmpDir := t.TempDir()
			target := newSignalingBuffer(ctx, tmpDir)

			mockClock := clock.NewMock()
			params.Interval = tc.interval
			params.Target = target
			params.Clock = mockClock
			params.DataType = tc.datatype
			c, err := NewCollector(tc.captureFunc, params)
			test.That(t, err, test.ShouldBeNil)
			c.Collect()
			// We need to avoid adding time until after the underlying goroutine has started sleeping.
			// If we add time before that point, data will never be captured, because time will never be greater than
			// the initially calculated time.
			// Sleeping for 10ms is a hacky way to ensure that we don't encounter this situation. It gives 10ms
			// for those few sequential lines in collector.go to execute, so that that occurs before we add time below.
			time.Sleep(10 * time.Millisecond)
			for i := 0; i < tc.expectReadings; i++ {
				mockClock.Add(params.Interval)
				select {
				case <-ctx.Done():
					t.Fatalf("timed out waiting for data to be written")
				case <-target.wrote:
				}
			}

			// If it's a sleep based collector, we need to move the clock forward one more time after calling Close.
			// Otherwise, it will stay asleep indefinitely and Close will block forever.
			// This loop guarantees that the clock is moved forward at least once after Close is called. After Close
			// returns and the closed channel is closed, this loop will terminate.
			closed := make(chan struct{})
			sleepCollector := tc.interval < sleepCaptureCutoff
			var wg sync.WaitGroup
			if sleepCollector {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < 1000; i++ {
						select {
						case <-closed:
							return
						default:
							time.Sleep(time.Millisecond * 1)
							mockClock.Add(params.Interval)
						}
					}
				}()
			}
			c.Close()
			close(closed)
			wg.Wait()

			var actReadings []*v1.SensorData
			files := getAllFiles(tmpDir)
			for _, file := range files {
				fileReadings, err := SensorDataFromCaptureFilePath(filepath.Join(tmpDir, file.Name()))
				test.That(t, err, test.ShouldBeNil)
				actReadings = append(actReadings, fileReadings...)
			}
			test.That(t, len(actReadings), test.ShouldEqual, tc.expectReadings)
			test.That(t, err, test.ShouldBeNil)
			validateReadings(t, actReadings, tc.expectReadings)
		})
	}
}

func TestClose(t *testing.T) {
	// Set up a collector.
	l := logging.NewTestLogger(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	tmpDir := t.TempDir()
	mockClock := clock.NewMock()
	target := newSignalingBuffer(ctx, tmpDir)
	interval := time.Millisecond * 5

	params := CollectorParams{
		DataType:      CaptureTypeTabular,
		ComponentName: "testComponent",
		Interval:      interval,
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		Target:        target,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        l,
		Clock:         mockClock,
	}
	c, err := NewCollector(structCapturer, params)
	test.That(t, err, test.ShouldBeNil)

	// Start collecting, and validate it is writing.
	c.Collect()
	mockClock.Add(interval)
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatalf("timed out waiting for data to be written")
	case <-target.wrote:
	}

	// Close and validate no additional writes occur even after an additional interval.
	c.Close()
	mockClock.Add(interval)
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()
	select {
	case <-ctx.Done():
	case <-target.wrote:
		t.Fatalf("unexpected write after close")
	}
}

// TestCtxCancelledNotLoggedAfterClose verifies that context cancelled errors are not logged if they occur after Close
// has been called. The collector context is cancelled as part of Close, so we expect to see context cancelled errors
// for any running capture routines.
func TestCtxCancelledNotLoggedAfterClose(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	tmpDir := t.TempDir()
	target := NewCaptureBuffer(tmpDir, &v1.DataCaptureMetadata{}, 50)
	captured := make(chan struct{})
	errorCapturer := CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (CaptureResult, error) {
		var res CaptureResult
		select {
		case <-ctx.Done():
			return res, fmt.Errorf("arbitrary wrapping message: %w", ctx.Err())
		case captured <- struct{}{}:
		}
		return dummyStructReading, nil
	})

	params := CollectorParams{
		ComponentName: "testComponent",
		DataType:      CaptureTypeTabular,
		Interval:      time.Millisecond,
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		Target:        target,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        logger,
	}
	c, err := NewCollector(errorCapturer, params)
	test.That(t, err, test.ShouldBeNil)
	c.Collect()
	<-captured
	c.Close()
	close(captured)

	failedLogs := logs.FilterLevelExact(zapcore.ErrorLevel)
	if failedLogs.Len() != 0 {
		// The test is going to fail. Output the logs for diagnostics.
		logger.Error("FailedLogs:", failedLogs)
	}
	test.That(t, failedLogs.Len(), test.ShouldEqual, 0)
}

func TestLogErrorsOnlyOnce(t *testing.T) {
	// Set up a collector.
	logger, logs := logging.NewObservedTestLogger(t)
	tmpDir := t.TempDir()
	errorCapturer := CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (CaptureResult, error) {
		return CaptureResult{}, errors.New("I am an error")
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	target := newSignalingBuffer(ctx, tmpDir)
	interval := time.Millisecond * 5

	mockClock := clock.NewMock()

	params := CollectorParams{
		DataType:      CaptureTypeTabular,
		ComponentName: "testComponent",
		Interval:      interval,
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		Target:        target,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        logger,
		Clock:         mockClock,
	}
	c, err := NewCollector(errorCapturer, params)
	test.That(t, err, test.ShouldBeNil)

	// Start collecting, and validate it is writing.
	c.Collect()
	mockClock.Add(interval * 5)

	test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldEqual, 1)
	mockClock.Add(3 * time.Second)
	test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldEqual, 2)
	mockClock.Add(3 * time.Second)
	test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldEqual, 3)
	c.Close()
}

func validateReadings(t *testing.T, act []*v1.SensorData, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		read := act[i]
		if read.GetStruct() != nil {
			test.That(t, proto.Equal(dummyStructReadingProto, read.GetStruct()), test.ShouldBeTrue)
		} else {
			test.That(t, read.GetBinary(), test.ShouldResemble, dummyBytesReading)
		}
	}
}

func getAllFiles(dir string) []os.FileInfo {
	var files []os.FileInfo
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		//nolint:nilerr
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, info)
		return nil
	})
	return files
}

func newSignalingBuffer(ctx context.Context, path string) *signalingBuffer {
	md := v1.DataCaptureMetadata{}
	return &signalingBuffer{
		ctx:   ctx,
		bw:    NewCaptureBuffer(path, &md, 50),
		wrote: make(chan struct{}),
	}
}

type signalingBuffer struct {
	ctx   context.Context
	bw    CaptureBufferedWriter
	wrote chan struct{}
}

func (b *signalingBuffer) WriteBinary(items []*v1.SensorData) error {
	ret := b.bw.WriteBinary(items)
	select {
	case b.wrote <- struct{}{}:
	case <-b.ctx.Done():
	}
	return ret
}

func (b *signalingBuffer) WriteTabular(item *v1.SensorData) error {
	ret := b.bw.WriteTabular(item)
	select {
	case b.wrote <- struct{}{}:
	case <-b.ctx.Done():
	}
	return ret
}

func (b *signalingBuffer) Flush() error {
	return b.bw.Flush()
}

func (b *signalingBuffer) Path() string {
	return b.bw.Path()
}
