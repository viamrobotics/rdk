package data

import (
	"context"
	"io/ioutil"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	"go.viam.com/test"

	pb "go.viam.com/rdk/proto/api/component/imu/v1"
)

type dummyCapturer struct {
	shouldError  bool
	captureCount int64
}

func (c *dummyCapturer) Capture(params map[string]string) (*any.Any, error) {
	if c.shouldError {
		return nil, errors.New("error")
	}

	atomic.AddInt64(&c.captureCount, 1)
	// Using an arbitrary proto message.
	return WrapProtoAll(&pb.ReadAccelerationRequest{Name: "name"}, nil)
}

func getFileSize(f *os.File) int64 {
	fileInfo, err := f.Stat()
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

// TODO: Don't do any checking on construction. Should we?
func TestNewCollector(t *testing.T) {
	c := NewCollector(nil, time.Second, nil, nil, nil)
	test.That(t, c, test.ShouldNotBeNil)
}

// TODO: once prefixed protobuf read/write is implemented, these tests should verify message contents, not just that the
//       file is being written to.
func TestSuccessfulWrite(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())

	// Verify that it writes to the file.
	c := NewCollector(&dummyCapturer{}, time.Millisecond*10, map[string]string{"name": "test"}, target1, l)
	go c.Collect(context.TODO())
	time.Sleep(time.Millisecond * 20)
	size1 := getFileSize(target1)
	test.That(t, size1, test.ShouldBeGreaterThan, 0)

	// Verify that it continues to write to the file.
	time.Sleep(time.Millisecond * 20)
	size2 := getFileSize(target1)
	test.That(t, size2, test.ShouldBeGreaterThan, size1)
}

func TestClose(t *testing.T) {
	// Set up a collector.
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	dummy := &dummyCapturer{}
	c := NewCollector(dummy, time.Millisecond*5, map[string]string{"name": "test"}, target1, l)
	go c.Collect(context.TODO())
	time.Sleep(time.Millisecond * 12)

	// Measure captureCount/fileSize.
	captureCount := atomic.LoadInt64(&dummy.captureCount)
	fileSize := getFileSize(target1)

	// Assert that after closing, capture is no longer being called and the file is not being written to.
	c.Close()
	time.Sleep(time.Millisecond * 10)
	test.That(t, atomic.LoadInt64(&dummy.captureCount), test.ShouldEqual, captureCount)
	test.That(t, getFileSize(target1), test.ShouldEqual, fileSize)
}

// Test that interval is respected and that capture() is called floor(time_passed/interval) times.
func TestInterval(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	dummy := &dummyCapturer{}
	c := NewCollector(dummy, time.Millisecond*5, map[string]string{"name": "test"}, target1, l)
	go c.Collect(context.TODO())

	// Give 2ms of leeway so slight changes in execution ordering don't impact the test.
	time.Sleep(time.Millisecond * 22)
	test.That(t, atomic.LoadInt64(&dummy.captureCount), test.ShouldEqual, 4)
}

func TestSetTarget(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever1")
	target2, _ := ioutil.TempFile("", "whatever2")
	defer os.Remove(target1.Name())
	defer os.Remove(target2.Name())

	dummy := &dummyCapturer{}
	c := NewCollector(dummy, time.Millisecond*5, map[string]string{"name": "test"}, target1, l)
	go c.Collect(context.TODO())
	time.Sleep(time.Millisecond * 12)

	// Measure fileSize of tgt1 and tgt2.
	oldSizeTgt1 := getFileSize(target1)
	oldSizeTgt2 := getFileSize(target2)
	test.That(t, oldSizeTgt1, test.ShouldBeGreaterThan, 0)
	test.That(t, oldSizeTgt2, test.ShouldEqual, 0)

	// Change target, let run for a bit.
	c.SetTarget(target2)
	time.Sleep(time.Millisecond * 12)
	c.Close()

	// Assert that file size of target 1 has not changed, and that target2 is now being written to.
	newSizeTgt1 := getFileSize(target1)
	newSizeTgt2 := getFileSize(target2)
	test.That(t, newSizeTgt1, test.ShouldEqual, oldSizeTgt1)
	test.That(t, newSizeTgt2, test.ShouldBeGreaterThan, 0)
}

// Verifies that Collect does not error if it receives a single error when calling capture.
func TestSwallowsErrors(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	dummy := &dummyCapturer{shouldError: true}

	c := NewCollector(dummy, time.Millisecond*5, map[string]string{"name": "test"}, target1, logger)
	errorChannel := make(chan error)
	defer close(errorChannel)
	go func() {
		err := c.Collect(context.TODO())
		if err != nil {
			errorChannel <- err
		}
	}()
	time.Sleep(10 * time.Millisecond)

	// Verify that no errors were passed into errorChannel, and that errors were logged.
	select {
	case err := <-errorChannel:
		logger.Fatalf("Collector.Collect propogated error: %s", err)
	default:
		test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldBeGreaterThan, 0)
	}
}
