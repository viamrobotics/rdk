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
	"go.uber.org/zap/zapcore"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager/datacapture"
)

type structReading struct {
	Field1 bool
}

func (r *structReading) toProto() *structpb.Struct {
	msg, err := protoutils.StructToStructPb(r)
	if err != nil {
		return nil
	}
	return msg
}

var (
	structCapturer = CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		return dummyStructReading, nil
	})
	binaryCapturer = CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		return dummyBytesReading, nil
	})
	dummyStructReading      = structReading{}
	dummyStructReadingProto = dummyStructReading.toProto()
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
		ComponentName: "name",
		Logger:        logging.NewTestLogger(t),
		Target:        datacapture.NewBuffer("dir", nil),
	})

	test.That(t, c2, test.ShouldNotBeNil)
	test.That(t, err2, test.ShouldBeNil)
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
	}{
		{
			name:           "Ticker based struct writer.",
			captureFunc:    structCapturer,
			interval:       tickerInterval,
			expectReadings: 2,
			expFiles:       1,
		},
		{
			name:           "Sleep based struct writer.",
			captureFunc:    structCapturer,
			interval:       sleepInterval,
			expectReadings: 2,
			expFiles:       1,
		},
		{
			name:           "Ticker based binary writer.",
			captureFunc:    binaryCapturer,
			interval:       tickerInterval,
			expectReadings: 2,
			expFiles:       2,
		},
		{
			name:           "Sleep based binary writer.",
			captureFunc:    binaryCapturer,
			interval:       sleepInterval,
			expectReadings: 2,
			expFiles:       2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
			defer cancel()
			tmpDir := t.TempDir()
			md := v1.DataCaptureMetadata{}
			tgt := datacapture.NewBuffer(tmpDir, &md)
			test.That(t, tgt, test.ShouldNotBeNil)
			wrote := make(chan struct{})
			target := &signalingBuffer{
				bw:    tgt,
				wrote: wrote,
			}

			mockClock := clock.NewMock()
			params.Interval = tc.interval
			params.Target = target
			params.Clock = mockClock
			c, err := NewCollector(tc.captureFunc, params)
			test.That(t, err, test.ShouldBeNil)
			c.Collect()
			// We need to avoid adding time until after the the underlying goroutine has started sleeping.
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
				case <-wrote:
				}
			}
			close(wrote)

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
							mockClock.Add(tc.interval)
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
				fileReadings, err := datacapture.SensorDataFromFilePath(filepath.Join(tmpDir, file.Name()))
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
	tmpDir := t.TempDir()
	md := v1.DataCaptureMetadata{}
	buf := datacapture.NewBuffer(tmpDir, &md)
	wrote := make(chan struct{})
	target := &signalingBuffer{
		bw:    buf,
		wrote: wrote,
	}
	mockClock := clock.NewMock()
	interval := time.Millisecond * 5

	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      interval,
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		Target:        target,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        l,
		Clock:         mockClock,
	}
	c, _ := NewCollector(structCapturer, params)

	// Start collecting, and validate it is writing.
	c.Collect()
	mockClock.Add(interval)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatalf("timed out waiting for data to be written")
	case <-wrote:
	}

	// Close and validate no additional writes occur even after an additional interval.
	c.Close()
	mockClock.Add(interval)
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()
	select {
	case <-ctx.Done():
	case <-wrote:
		t.Fatalf("unexpected write after close")
	}
}

// TestCtxCancelledNotLoggedAfterClose verifies that context cancelled errors are not logged if they occur after Close
// has been called. The collector context is cancelled as part of Close, so we expect to see context cancelled errors
// for any running capture routines.
func TestCtxCancelledNotLoggedAfterClose(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	tmpDir := t.TempDir()
	target := datacapture.NewBuffer(tmpDir, &v1.DataCaptureMetadata{})
	captured := make(chan struct{})
	errorCapturer := CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("arbitrary wrapping message: %w", ctx.Err())
		case captured <- struct{}{}:
		}
		return dummyStructReading, nil
	})

	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      time.Millisecond * 1,
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		Target:        target,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        logger,
	}
	c, _ := NewCollector(errorCapturer, params)
	c.Collect()
	<-captured
	c.Close()
	close(captured)

	test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldEqual, 0)
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

//nolint
func getAllFiles(dir string) []os.FileInfo {
	var files []os.FileInfo
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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

type signalingBuffer struct {
	bw    datacapture.BufferedWriter
	wrote chan struct{}
}

func (b *signalingBuffer) Write(data *v1.SensorData) error {
	ret := b.bw.Write(data)
	b.wrote <- struct{}{}
	return ret
}

func (b *signalingBuffer) Flush() error {
	return b.bw.Flush()
}

func (b *signalingBuffer) Path() string {
	return b.bw.Path()
}
