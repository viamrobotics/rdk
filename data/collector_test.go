package data

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/protoutils"
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
	dummyStructCapturer = CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		return dummyStructReading, nil
	})
	dummyBinaryCapturer = CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		return dummyBytesReading, nil
	})
	dummyStructReading      = structReading{}
	dummyStructReadingProto = dummyStructReading.toProto()
	dummyBytesReading       = []byte("I sure am bytes")
	queueSize               = 250
	bufferSize              = 4096
)

func TestNewCollector(t *testing.T) {
	// If missing parameters should return an error.
	c1, err1 := NewCollector(nil, CollectorParams{})

	test.That(t, c1, test.ShouldBeNil)
	test.That(t, err1, test.ShouldNotBeNil)

	// If not missing parameters, should not return an error.
	target1, _ := ioutil.TempFile("", "whatever")
	c2, err2 := NewCollector(nil, CollectorParams{
		ComponentName: "name",
		Logger:        golog.NewTestLogger(t),
		Target:        target1,
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
				MethodParams:  map[string]string{"name": "test"},
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
				MethodParams:  map[string]string{"name": "test"},
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
				MethodParams:  map[string]string{"name": "test"},
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
				MethodParams:  map[string]string{"name": "test"},
				QueueSize:     queueSize,
				BufferSize:    bufferSize,
				Logger:        l,
			},
			wait:           sleepInterval*time.Duration(2) + sleepInterval/time.Duration(2),
			expectReadings: 2,
		},
	}

	for _, tc := range tests {
		target, _ := ioutil.TempFile("", "whatever")
		tc.params.Target = target
		c, _ := NewCollector(tc.capturer, tc.params)
		go c.Collect()

		// Verify that it writes to the file at all.
		time.Sleep(tc.wait)
		c.Close()
		fileSize := getFileSize(target)
		test.That(t, fileSize, test.ShouldBeGreaterThan, 0)

		// Verify that the data it wrote matches what we expect.
		validateReadings(t, target, tc.expectReadings)

		// Next reading should fail; there should be at most max readings.
		_, err := readNextSensorData(target)
		test.That(t, err, test.ShouldEqual, io.EOF)
		os.Remove(target.Name())
	}
}

func TestClose(t *testing.T) {
	// Set up a collector.
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      time.Millisecond * 15,
		MethodParams:  map[string]string{"name": "test"},
		Target:        target1,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        l,
	}
	c, _ := NewCollector(dummyStructCapturer, params)
	go c.Collect()
	time.Sleep(time.Millisecond * 50)

	// Close and measure fileSize.
	c.Close()
	fileSize := getFileSize(target1)

	// Assert capture is no longer being called/file is no longer being written to.
	time.Sleep(time.Millisecond * 25)
	test.That(t, getFileSize(target1), test.ShouldEqual, fileSize)
}

func TestSetTarget(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever1")
	target2, _ := ioutil.TempFile("", "whatever2")
	defer os.Remove(target1.Name())
	defer os.Remove(target2.Name())

	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      time.Millisecond * 15,
		MethodParams:  map[string]string{"name": "test"},
		Target:        target1,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        l,
	}
	c, _ := NewCollector(dummyStructCapturer, params)
	go c.Collect()
	time.Sleep(time.Millisecond * 30)

	// Change target, verify that target1 was written to.
	c.SetTarget(target2)
	sizeTgt1 := getFileSize(target1)
	test.That(t, getFileSize(target1), test.ShouldBeGreaterThan, 0)

	// Verify that tgt2 was written to, and that target1 was not written to after the target was changed.
	time.Sleep(time.Millisecond * 30)
	c.Close()
	test.That(t, getFileSize(target1), test.ShouldEqual, sizeTgt1)
	test.That(t, getFileSize(target2), test.ShouldBeGreaterThan, 0)
}

// Verifies that Collect does not error if it receives a single error when calling capture, and that those errors are
// logged.
func TestSwallowsErrors(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())

	errorCapturer := CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		return nil, errors.New("error")
	})
	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      time.Millisecond * 10,
		MethodParams:  map[string]string{"name": "test"},
		Target:        target1,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        logger,
	}
	c, _ := NewCollector(errorCapturer, params)
	errorChannel := make(chan error)
	defer close(errorChannel)
	go func() {
		err := c.Collect()
		if err != nil {
			errorChannel <- err
		}
	}()
	time.Sleep(30 * time.Millisecond)
	c.Close()

	// Sleep for a short period to avoid race condition when accessing the logs below (since the collector might still
	// write an error log for a few instructions after .Close() is called, and this test is reading from the logger).
	time.Sleep(10 * time.Millisecond)

	// Verify that no errors were passed into errorChannel, and that errors were logged.
	select {
	case err := <-errorChannel:
		logger.Fatalf("Collector.Collect propogated error: %s", err)
	default:
		test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldBeGreaterThan, 0)
	}
}

// TestCtxCancelledLoggedAsDebug verifies that context cancelled errors are logged as debug level instead of as errors.
func TestCtxCancelledLoggedAsDebug(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	errorCapturer := CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		return nil, fmt.Errorf("arbitrary wrapping message: %w", context.Canceled)
	})
	params := CollectorParams{
		ComponentName: "testComponent",
		Interval:      time.Millisecond * 10,
		MethodParams:  map[string]string{"name": "test"},
		Target:        target1,
		QueueSize:     queueSize,
		BufferSize:    bufferSize,
		Logger:        logger,
	}
	c, _ := NewCollector(errorCapturer, params)
	go c.Collect()
	time.Sleep(50 * time.Millisecond)
	c.Close()

	// Sleep for a short period to avoid race condition when accessing the logs below (since the collector might still
	// write an error log for a few instructions after .Close() is called, and this test is reading from the logger).
	time.Sleep(10 * time.Millisecond)

	test.That(t, logs.FilterLevelExact(zapcore.DebugLevel).Len(), test.ShouldBeGreaterThan, 0)
	test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldEqual, 0)
}

func validateReadings(t *testing.T, file *os.File, n int) {
	t.Helper()
	_, _ = file.Seek(0, 0)
	for i := 0; i < n; i++ {
		read, err := readNextSensorData(file)
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

func getFileSize(f *os.File) int64 {
	fileInfo, err := f.Stat()
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

func readNextSensorData(f *os.File) (*v1.SensorData, error) {
	r := &v1.SensorData{}
	if _, err := pbutil.ReadDelimited(f, r); err != nil {
		return nil, err
	}
	return r, nil
}
