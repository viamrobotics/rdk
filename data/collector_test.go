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
	c := NewCollector(&dummyCapturer{}, time.Millisecond*10, map[string]string{"name": "test"}, target1, l)
	go c.Collect(context.Background())
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
	go c.Collect(context.Background())
	time.Sleep(time.Millisecond * 25)

	// Measure captureCount/fileSize.
	captureCount := atomic.LoadInt64(&dummy.captureCount)
	c.Close()
	fileSize := getFileSize(target1)

	// Assert capture is no longer being called and the file is not being written to.
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
	// floor(83/5) = 16
	time.Sleep(time.Millisecond * 83)
	test.That(t, atomic.LoadInt64(&dummy.captureCount), test.ShouldEqual, 16)
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

func getFileSize(f *os.File) int64 {
	fileInfo, err := f.Stat()
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}
