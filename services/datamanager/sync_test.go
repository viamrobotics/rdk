package datamanager

import (
	"context"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
)

func newTestSyncer(t *testing.T, uploadFn func(ctx context.Context, path string) error) syncer {
	t.Helper()
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	captureDir := t.TempDir()
	l := golog.NewTestLogger(t)

	return syncer{
		captureDir: captureDir,
		logger:     l,
		progressTracker: progressTracker{
			lock: &sync.Mutex{},
			m:    make(map[string]struct{}),
		},
		uploadFn:   uploadFn,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFn,
	}
}

// Validates that Start uploads files currently in captureDir.
func TestStart(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(sut.captureDir, "whatever2")
	defer os.Remove(file2.Name())
	// Start syncer.
	sut.Start()
	// Give it some time to run and upload files.
	time.Sleep(time.Millisecond * 500)

	// Verify files were  uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}

func TestSync(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(sut.captureDir, "whatever2")
	defer os.Remove(file2.Name())
	sut.Sync([]string{file1.Name(), file2.Name()})
	time.Sleep(time.Millisecond * 500)

	// Verify files were  uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}

func TestOnlyUploadsOnce(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		time.Sleep(time.Millisecond * 500)
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(sut.captureDir, "whatever2")
	defer os.Remove(file2.Name())
	// Start syncer, twice.
	sut.Start()
	sut.Start()
	// Immediately try to Sync same files, twice.
	sut.Sync([]string{file1.Name(), file2.Name()})
	sut.Sync([]string{file1.Name(), file2.Name()})

	// Verify upload was only called twice.
	time.Sleep(time.Second * 1)
	// Verify files were  uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}

// Validates that if a syncer is killed after enqueing a file, a new syncer will still pick it up and upload it.
// This is to simulate the case where a robot is killed mid-sync; we still want that sync to resume and finish when it
// turns back on.
func TestRecoversAfterKilled(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Put a file in syncDir; this simulates a file that was enqueued by some previous syncer.
	file1, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file1.Name())

	// Put a file in captureDir; this simulates a file that was written but not yet queued by some previous syncer.
	// It should be synced even if it is not specified in the list passed to Enqueue.
	file2, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file2.Name())

	// Start syncer, let it run for a second.
	sut.Start()
	time.Sleep(time.Second)

	// Verify files were uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in syncDir")
	}

	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}

func TestUploadExponentialRetry(t *testing.T) {
	// Define an uploadFunc that fails 4 times then succeeds on its 5th attempt.
	failureCount := 0
	successCount := 0
	callTimes := make(map[int]time.Time)
	uploadFunc := func(ctx context.Context, path string) error {
		callTimes[failureCount+successCount] = time.Now()
		if failureCount >= 4 {
			successCount++
			return nil
		}
		failureCount++
		return errors.New("fail for the first 4 tries, then succeed")
	}
	sut := newTestSyncer(t, uploadFunc)

	// Put a file to be synced in captureDir.
	file1, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file1.Name())

	// Start syncer and let it run.
	initialWaitTime = time.Millisecond * 25
	maxRetryInterval = time.Millisecond * 150
	sut.Start()
	time.Sleep(time.Second)
	sut.Close()

	// Test that upload failed 4 times then succeeded once.
	test.That(t, failureCount, test.ShouldEqual, 4)
	test.That(t, successCount, test.ShouldEqual, 1)

	// Test that exponential increase happens.
	// First retry should wait initialWaitTime
	// Give some leeway so small variations in timing don't cause test failures.
	marginOfError := time.Millisecond * 20
	test.That(t, callTimes[1].Sub(callTimes[0]), test.ShouldAlmostEqual, initialWaitTime, marginOfError)

	// Then increase by a factor of retryExponentialFactor each time
	test.That(t, callTimes[2].Sub(callTimes[1]), test.ShouldAlmostEqual,
		initialWaitTime*time.Duration(retryExponentialFactor), marginOfError)
	test.That(t, callTimes[3].Sub(callTimes[2]), test.ShouldAlmostEqual,
		initialWaitTime*time.Duration(retryExponentialFactor*retryExponentialFactor), marginOfError)

	// ... but not increase past maxRetryInterval.
	test.That(t, callTimes[4].Sub(callTimes[3]), test.ShouldAlmostEqual, maxRetryInterval, marginOfError)
}
