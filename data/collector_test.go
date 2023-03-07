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

	"github.com/edaniels/golog"
	"go.uber.org/zap/zapcore"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

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
		Logger:        golog.NewTestLogger(t),
		Target:        datacapture.NewBuffer("dir", nil),
	})

	test.That(t, c2, test.ShouldNotBeNil)
	test.That(t, err2, test.ShouldBeNil)
}

// Test that the Collector correctly writes the SensorData on an interval.
func TestSuccessfulWrite(t *testing.T) {
	l := golog.NewTestLogger(t)
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
			// TODO: figure out how to avoid this. Don't want to do below until .Collect goroutine has kicked off.
			// Can make Collect just not kick off a goroutine, and make it's caller do that? Maybe the better pattern
			time.Sleep(time.Millisecond * 10)
			for i := 0; i < tc.expectReadings; i++ {
				mockClock.Add(params.Interval)
				<-wrote
			}
			close(wrote)

			// If it's a sleep based collector, we need to move the clock forward one more time after calling Close.
			// Otherwise, it will stay asleep indefinitely and Close will block forever.
			// This loop guarantees that the clock is moved forward at least once after Close is called. After Close
			// returns and the closed channel is closed, this loop will terminate.
			closed := make(chan struct{})
			sleepCollector := tc.interval < sleepCaptureCutoff
			wg := sync.WaitGroup{}
			if sleepCollector {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < 100; i++ {
						select {
						case <-closed:
							return
						default:
							time.Sleep(time.Millisecond * 10)
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
	l := golog.NewTestLogger(t)
	tmpDir := t.TempDir()
	md := v1.DataCaptureMetadata{}
	target := datacapture.NewBuffer(tmpDir, &md)
	sleepCaptureCutoff = time.Millisecond * 10

	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      time.Millisecond * 5,
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		Target:        target,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        l,
	}
	c, _ := NewCollector(structCapturer, params)
	c.Collect()
	time.Sleep(time.Millisecond * 25)

	// Close and measure fileSize.
	c.Close()
	files := getAllFiles(target.Directory)
	test.That(t, len(files), test.ShouldEqual, 1)
	fileSize1 := files[0].Size()

	// Assert capture is no longer being called/file is no longer being written to.
	time.Sleep(time.Millisecond * 25)
	filesAfterWait := getAllFiles(target.Directory)
	test.That(t, len(filesAfterWait), test.ShouldEqual, 1)
	test.That(t, filesAfterWait[0].Size(), test.ShouldEqual, fileSize1)
}

// TestCtxCancelledLoggedAsDebug verifies that context cancelled errors are logged as debug level instead of as errors.
func TestCtxCancelledLoggedAsDebug(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)
	tmpDir := t.TempDir()
	target := datacapture.NewBuffer(tmpDir, &v1.DataCaptureMetadata{})
	captured := make(chan struct{})
	errorCapturer := CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		select {
		case <-ctx.Done():
		case captured <- struct{}{}:
		}
		return nil, fmt.Errorf("arbitrary wrapping message: %w", context.Canceled)
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

	test.That(t, logs.FilterLevelExact(zapcore.DebugLevel).Len(), test.ShouldBeGreaterThan, 0)
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

// nolint
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
