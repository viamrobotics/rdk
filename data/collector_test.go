package data

import (
	"context"
	"io/ioutil"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "go.viam.com/rdk/proto/api/service/datamanager/v1"
)

type exampleReading struct {
	Field1 bool
}

func (r *exampleReading) toProto() *structpb.Struct {
	msg, err := InterfaceToStruct(r)
	if err != nil {
		return nil
	}
	return msg
}

var (
	dummyReading      = exampleReading{}
	dummyReadingProto = dummyReading.toProto()
)

type dummyCapturer struct {
	ShouldError  bool
	CaptureCount int64
}

func (c *dummyCapturer) Capture(_ context.Context, _ map[string]string) (interface{}, error) {
	if c.ShouldError {
		return nil, errors.New("error")
	}

	atomic.AddInt64(&c.CaptureCount, 1)
	return dummyReading, nil
}

func TestNewCollector(t *testing.T) {
	c := NewCollector(nil, time.Second, nil, nil, nil)
	test.That(t, c, test.ShouldNotBeNil)
}

func TestSuccessfulWrite(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())

	// Verify that it writes to the file.
	c := NewCollector(&dummyCapturer{}, time.Millisecond*10, map[string]string{"name": "test"}, target1, l)
	go c.Collect()
	time.Sleep(time.Millisecond * 20)
	c.Close()
	fileSize := getFileSize(target1)
	test.That(t, fileSize, test.ShouldBeGreaterThan, 0)

	// Verify that the data it wrote matches what we expect.
	_, _ = target1.Seek(0, 0)
	read, err := readNextSensorData(target1)
	if err != nil {
		t.Fatalf("failed to read SensorData from file: %v", err)
	}

	test.That(t, proto.Equal(dummyReadingProto, read.Data), test.ShouldBeTrue)
}

func TestClose(t *testing.T) {
	// Set up a collector.
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	dummy := &dummyCapturer{}
	c := NewCollector(dummy, time.Millisecond*15, map[string]string{"name": "test"}, target1, l)
	go c.Collect()
	time.Sleep(time.Millisecond * 25)

	// Measure CaptureCount/fileSize.
	captureCount := atomic.LoadInt64(&dummy.CaptureCount)
	c.Close()
	fileSize := getFileSize(target1)

	// Assert capture is no longer being called and the file is not being written to.
	time.Sleep(time.Millisecond * 25)
	test.That(t, atomic.LoadInt64(&dummy.CaptureCount), test.ShouldEqual, captureCount)
	test.That(t, getFileSize(target1), test.ShouldEqual, fileSize)
}

// Test that interval is respected and that capture() is called floor(time_passed/interval) times.
func TestInterval(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	dummy := &dummyCapturer{}
	c := NewCollector(dummy, time.Millisecond*25, map[string]string{"name": "test"}, target1, l)
	go c.Collect()

	// Give 20ms of leeway so slight changes in execution ordering don't impact the test.
	// floor(70/25) = 2
	time.Sleep(time.Millisecond * 70)
	test.That(t, atomic.LoadInt64(&dummy.CaptureCount), test.ShouldEqual, 2)
}

func TestSetTarget(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever1")
	target2, _ := ioutil.TempFile("", "whatever2")
	defer os.Remove(target1.Name())
	defer os.Remove(target2.Name())

	dummy := &dummyCapturer{}
	c := NewCollector(dummy, time.Millisecond*20, map[string]string{"name": "test"}, target1, l)
	go c.Collect()

	// Let it write to tgt1 for a bit.
	time.Sleep(time.Millisecond * 25)

	// Change target, let run for a bit.
	c.SetTarget(target2)
	time.Sleep(time.Millisecond * 25)

	// Verify tgt1 and tgt2 were written to, and that any buffered data was flushed when the target was changed.
	c.Close()
	sizeTgt1 := getFileSize(target1)
	sizeTgt2 := getFileSize(target2)
	test.That(t, sizeTgt1, test.ShouldBeGreaterThan, 0)
	test.That(t, sizeTgt2, test.ShouldBeGreaterThan, 0)
}

// Verifies that Collect does not error if it receives a single error when calling capture, and that those errors are
// logged.
func TestSwallowsErrors(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	dummy := &dummyCapturer{ShouldError: true}

	c := NewCollector(dummy, time.Millisecond*10, map[string]string{"name": "test"}, target1, logger)
	errorChannel := make(chan error)
	defer close(errorChannel)
	go func() {
		err := c.Collect()
		if err != nil {
			errorChannel <- err
		}
	}()
	time.Sleep(30 * time.Millisecond)

	// Verify that no errors were passed into errorChannel, and that errors were logged.
	select {
	case err := <-errorChannel:
		logger.Fatalf("Collector.Collect propogated error: %s", err)
	default:
		test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldBeGreaterThan, 0)
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
