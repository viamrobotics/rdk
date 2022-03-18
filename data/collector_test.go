package data

import (
	"context"
	"io/ioutil"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	"go.viam.com/test"
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
	return c, nil
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
	c := NewCollector(&dummyCapturer{}, time.Millisecond*10, map[string]string{"name": "test"}, target1, l)
	go c.Collect()
	time.Sleep(time.Millisecond * 20)
	c.Close()
	test.That(t, getFileSize(target1), test.ShouldBeGreaterThan, 0)
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
	c := NewCollector(dummy, time.Millisecond*10, map[string]string{"name": "test"}, target1, l)
	go c.Collect()

	// Give 5ms of leeway so slight changes in execution ordering don't impact the test.
	// floor(85/10) = 8
	time.Sleep(time.Millisecond * 85)
	test.That(t, atomic.LoadInt64(&dummy.CaptureCount), test.ShouldEqual, 8)
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

	c := NewCollector(dummy, time.Millisecond*20, map[string]string{"name": "test"}, target1, logger)
	errorChannel := make(chan error)
	defer close(errorChannel)
	go func() {
		err := c.Collect()
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

func getFileSize(f *os.File) int64 {
	fileInfo, err := f.Stat()
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}
