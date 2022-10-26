package data

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

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
	dummyStructCapturer = CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		return dummyStructReading, nil
	})
	dummyBinaryCapturer = CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
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
		Target:        &datacapture.File{},
	})

	test.That(t, c2, test.ShouldNotBeNil)
	test.That(t, err2, test.ShouldBeNil)
}

// Test that SensorData is written correctly and can be read, and that interval is respected and that capture()
// is called floor(time_passed/interval) times in the ticker (interval >= 2ms) case.
func TestSuccessfulWrite(t *testing.T) {
	l := golog.NewTestLogger(t)
	// Set sleepIntervalCutoff high, because tests using the prod cutoff (2ms) have enough variation in timing
	// to make them flaky.
	sleepCaptureCutoff = time.Millisecond * 100
	tickerInterval := time.Millisecond * 101
	sleepInterval := time.Millisecond * 99

	tests := []struct {
		name           string
		capturer       Capturer
		params         CollectorParams
		wait           time.Duration
		expectReadings int
	}{
		{
			name:     "Ticker based struct writer.",
			capturer: dummyStructCapturer,
			params: CollectorParams{
				ComponentName: "testComponent",
				Interval:      tickerInterval,
				MethodParams:  map[string]*anypb.Any{"name": fakeVal},
				QueueSize:     queueSize,
				BufferSize:    bufferSize,
				Logger:        l,
			},
			wait:           tickerInterval*time.Duration(2) + tickerInterval/time.Duration(2),
			expectReadings: 2,
		},
		{
			name:     "Sleep based struct writer.",
			capturer: dummyStructCapturer,
			params: CollectorParams{
				ComponentName: "testComponent",
				Interval:      sleepInterval,
				MethodParams:  map[string]*anypb.Any{"name": fakeVal},
				QueueSize:     queueSize,
				BufferSize:    bufferSize,
				Logger:        l,
			},
			wait:           sleepInterval*time.Duration(2) + sleepInterval/time.Duration(2),
			expectReadings: 2,
		},
		{
			name:     "Ticker based binary writer.",
			capturer: dummyBinaryCapturer,
			params: CollectorParams{
				ComponentName: "testComponent",
				Interval:      tickerInterval,
				MethodParams:  map[string]*anypb.Any{"name": fakeVal},
				QueueSize:     queueSize,
				BufferSize:    bufferSize,
				Logger:        l,
			},
			wait:           tickerInterval*time.Duration(2) + tickerInterval/time.Duration(2),
			expectReadings: 2,
		},
		{
			name:     "Sleep based binary writer.",
			capturer: dummyBinaryCapturer,
			params: CollectorParams{
				ComponentName: "testComponent",
				Interval:      sleepInterval,
				MethodParams:  map[string]*anypb.Any{"name": fakeVal},
				QueueSize:     queueSize,
				BufferSize:    bufferSize,
				Logger:        l,
			},
			wait:           sleepInterval*time.Duration(2) + sleepInterval/time.Duration(2),
			expectReadings: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := os.TempDir()
			md := v1.DataCaptureMetadata{}
			target, err := datacapture.NewFile(tmpDir, &md)
			test.That(t, err, test.ShouldBeNil)

			tc.params.Target = target
			c, err := NewCollector(tc.capturer, tc.params)
			test.That(t, err, test.ShouldBeNil)
			c.Collect()

			// Verify that it writes to the file at all.
			time.Sleep(tc.wait)
			c.Close()
			fileSize := target.Size()
			test.That(t, fileSize, test.ShouldBeGreaterThan, 0)

			// Verify that the data it wrote matches what we expect.
			validateReadings(t, target.GetPath(), tc.expectReadings)

			// Next reading should fail; there should be at most max readings.
			_, err = target.ReadNext()
			test.That(t, err, test.ShouldEqual, io.EOF)
			os.Remove(target.GetPath())
		})
	}
}

func TestClose(t *testing.T) {
	// Set up a collector.
	l := golog.NewTestLogger(t)
	tmpDir := os.TempDir()
	md := v1.DataCaptureMetadata{}
	target1, _ := datacapture.NewFile(tmpDir, &md)
	defer os.Remove(target1.GetPath())
	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      time.Millisecond * 15,
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		Target:        target1,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        l,
	}
	c, _ := NewCollector(dummyStructCapturer, params)
	c.Collect()
	time.Sleep(time.Millisecond * 25)

	// Close and measure fileSize.
	c.Close()
	fileSize := target1.Size()

	// Assert capture is no longer being called/file is no longer being written to.
	time.Sleep(time.Millisecond * 25)
	test.That(t, target1.Size(), test.ShouldEqual, fileSize)
}

func TestSetTarget(t *testing.T) {
	l := golog.NewTestLogger(t)
	tmpDir := os.TempDir()
	md1 := v1.DataCaptureMetadata{
		ComponentName: "someFirstThing",
	}
	md2 := v1.DataCaptureMetadata{
		ComponentName: "someSecondThing",
	}
	target1, _ := datacapture.NewFile(tmpDir, &md1)
	target2, _ := datacapture.NewFile(tmpDir, &md2)
	defer os.Remove(target1.GetPath())
	defer os.Remove(target2.GetPath())

	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      time.Millisecond * 15,
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		Target:        target1,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        l,
	}
	c, _ := NewCollector(dummyStructCapturer, params)
	c.Collect()
	time.Sleep(time.Millisecond * 30)

	// Change target, verify that target1 was written to.
	c.SetTarget(target2)
	sizeTgt1 := target1.Size()
	test.That(t, target1.Size(), test.ShouldBeGreaterThan, 0)

	// Verify that tgt2 was written to, and that target1 was not written to after the target was changed.
	time.Sleep(time.Millisecond * 30)
	c.Close()
	test.That(t, target1.Size(), test.ShouldEqual, sizeTgt1)
	test.That(t, target2.Size(), test.ShouldBeGreaterThan, 0)
}

// TestCtxCancelledLoggedAsDebug verifies that context cancelled errors are logged as debug level instead of as errors.
func TestCtxCancelledLoggedAsDebug(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)
	tmpDir := os.TempDir()
	md := v1.DataCaptureMetadata{}
	target1, _ := datacapture.NewFile(tmpDir, &md)
	defer os.Remove(target1.GetPath())
	errorCapturer := CaptureFunc(func(ctx context.Context, _ map[string]*anypb.Any) (interface{}, error) {
		return nil, fmt.Errorf("arbitrary wrapping message: %w", context.Canceled)
	})

	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      time.Millisecond * 10,
		MethodParams:  map[string]*anypb.Any{"name": fakeVal},
		Target:        target1,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        logger,
	}
	c, _ := NewCollector(errorCapturer, params)
	c.Collect()
	time.Sleep(25 * time.Millisecond)
	c.Close()

	// Sleep for a short period to avoid race condition when accessing the logs below (since the collector might still
	// write an error log for a few instructions after .Close() is called, and this test is reading from the logger).
	time.Sleep(10 * time.Millisecond)

	test.That(t, logs.FilterLevelExact(zapcore.DebugLevel).Len(), test.ShouldBeGreaterThan, 0)
	test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldEqual, 0)
}

func validateReadings(t *testing.T, filePath string, n int) {
	t.Helper()
	file, err := os.Open(filePath)
	test.That(t, err, test.ShouldBeNil)
	f, err := datacapture.ReadFile(file)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < n; i++ {
		read, err := f.ReadNext()
		if err != nil {
			t.Fatalf("failed to read SensorData from file: %v", err)
		}
		if read.GetStruct() != nil {
			test.That(t, proto.Equal(dummyStructReadingProto, read.GetStruct()), test.ShouldBeTrue)
		} else {
			test.That(t, read.GetBinary(), test.ShouldResemble, dummyBytesReading)
		}
	}
}
