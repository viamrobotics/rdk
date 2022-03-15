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

func (c *dummyCapturer) Capture(_ map[string]string) (*any.Any, error) {
	if c.shouldError {
		return nil, errors.New("error")
	}

	atomic.AddInt64(&c.captureCount, 1)
	// Using an arbitrary proto message.
	return WrapInAll(&pb.ReadAccelerationRequest{Name: "name"}, nil)
}

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
	c := NewCollector(&dummyCapturer{}, time.Millisecond*20, map[string]string{"name": "test"}, target1, l)
	go c.Collect(context.TODO())
	time.Sleep(time.Millisecond * 25)
	size1 := getFileSizeOfTarget(c)
	test.That(t, size1, test.ShouldBeGreaterThan, 0)

	// Verify that it continues to write to the file, and that Close() causes buffered messages to be written.
	time.Sleep(time.Millisecond * 25)
	c.Close()
	test.That(t, getFileSize(target1), test.ShouldBeGreaterThan, size1)
}

func TestClose(t *testing.T) {
	// Set up a collector.
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	dummy := &dummyCapturer{}
	c := NewCollector(dummy, time.Millisecond*15, map[string]string{"name": "test"}, target1, l)
	go c.Collect(context.Background())
	time.Sleep(time.Millisecond * 25)

	// Measure captureCount/fileSize.
	captureCount := atomic.LoadInt64(&dummy.captureCount)
	fileSize := getFileSizeOfTarget(c)

	// Assert that after closing, capture is no longer being called and the file is not being written to.
	c.Close()
	time.Sleep(time.Millisecond * 25)
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
	go c.Collect(context.Background())

	// Give 3ms of leeway so slight changes in execution ordering don't impact the test.
	// floor(43/5) = 8
	time.Sleep(time.Millisecond * 43)
	test.That(t, atomic.LoadInt64(&dummy.captureCount), test.ShouldEqual, 8)
}

func TestSetTarget(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever1")
	target2, _ := ioutil.TempFile("", "whatever2")
	defer os.Remove(target1.Name())
	defer os.Remove(target2.Name())

	dummy := &dummyCapturer{}
	c := NewCollector(dummy, time.Millisecond*20, map[string]string{"name": "test"}, target1, l)
	go c.Collect(context.Background())
	time.Sleep(time.Millisecond * 25)

	// Verify tgt1 is being written to.
	oldSizeTgt1 := getFileSizeOfTarget(c)
	test.That(t, oldSizeTgt1, test.ShouldBeGreaterThan, 0)

	// Change target, let run for a bit.
	c.SetTarget(target2)
	time.Sleep(time.Millisecond * 25)
	c.Close()

	// Assert that file size of target 1 has not changed, and that target2 is now being written to.
	newSizeTgt1 := getFileSize(target1)
	newSizeTgt2 := getFileSize(target2)
	test.That(t, newSizeTgt1, test.ShouldEqual, oldSizeTgt1)
	test.That(t, newSizeTgt2, test.ShouldBeGreaterThan, 0)
}

// Verifies that Collect does not error if it receives a single error when calling capture, and that those errors are
// logged.
func TestSwallowsErrors(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	dummy := &dummyCapturer{shouldError: true}

	c := NewCollector(dummy, time.Millisecond*20, map[string]string{"name": "test"}, target1, logger)
	errorChannel := make(chan error)
	defer close(errorChannel)
	go func() {
		err := c.Collect(context.Background())
		if err != nil {
			errorChannel <- err
		}
	}()
	time.Sleep(25 * time.Millisecond)

	// Verify that no errors were passed into errorChannel, and that errors were logged.
	select {
	case err := <-errorChannel:
		logger.Fatalf("Collector.Collect propogated error: %s", err)
	default:
		test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldBeGreaterThan, 0)
	}
}

// Convenience method for getting the file size of a given collectors target. It's used because the Collector uses a
// bufio.Writer, so some data might not be written to disk yet if c.Close() has not been called.
func getFileSizeOfTarget(c Collector) int64 {
	c.lock.Lock()
	defer c.lock.Unlock()
	return getFileSize(c.target) + int64(c.writer.Buffered())
}

func getFileSize(f *os.File) int64 {
	fileInfo, err := f.Stat()
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}
