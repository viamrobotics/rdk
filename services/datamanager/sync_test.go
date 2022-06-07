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
	syncQueue := t.TempDir()
	l := golog.NewTestLogger(t)

	return syncer{
		captureDir:    captureDir,
		syncQueue:     syncQueue,
		queueLock:     &sync.Mutex{},
		logger:        l,
		queueWaitTime: time.Nanosecond,
		progressTracker: progressTracker{
			lock: &sync.Mutex{},
			m:    make(map[string]struct{}),
		},
		uploadFn:   uploadFn,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFn,
	}
}

// Validates that for some captureDir, files are enqueued and uploaded exactly once on the scheduled syncer.

func TestQueuesAndUploadsOnceScheduled(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Start syncer, pass in its own cancel context as it's the same as the test's.
	sut.initialQueue()

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(sut.captureDir, "file1")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(sut.captureDir, "file2")
	defer os.Remove(file2.Name())
	err := sut.Queue([]string{file1.Name(), file2.Name()})
	test.That(t, err, test.ShouldBeNil)
	// Give it a second to run and upload files.
	// Queue/upload with the syncer, let it run for a second.
	err = sut.Upload()
	time.Sleep(time.Second)
	test.That(t, err, test.ShouldBeNil)

	// Verify files were enqueued and uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueue, err := ioutil.ReadDir(sut.syncQueue)
	if err != nil {
		t.Fatalf("failed to list files in syncQueue")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}

// Validates that for some captureDir, files are enqueued and uploaded exactly once on the manual syncer.
func TestQueuesAndUploadsOnceManual(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Put a couple files in captureDir, then put them in the sync queue and upload them.
	file1, _ := ioutil.TempFile(sut.captureDir, "file1")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(sut.captureDir, "file2")
	defer os.Remove(file2.Name())
	err := sut.Queue([]string{file1.Name(), file2.Name()})
	test.That(t, err, test.ShouldBeNil)
	err = sut.Upload()
	test.That(t, err, test.ShouldBeNil)
	// Give it a second to run and upload files.
	time.Sleep(time.Second)

	// Verify files were enqueued and uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueue, err := ioutil.ReadDir(sut.syncQueue)
	if err != nil {
		t.Fatalf("failed to list files in syncQueue")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}

// Validates that we can enqueue files without duplicates with both the manual syncer and scheduled syncer trying to queue.
func TestManualThenScheduledSync(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Put a couple files in captureDir, then have the manual + scheduled syncer put them in the sync queue.
	file1, _ := ioutil.TempFile(sut.captureDir, "file1")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(sut.captureDir, "file2")
	defer os.Remove(file2.Name())
	err := sut.Queue([]string{file1.Name(), file2.Name()})
	test.That(t, err, test.ShouldBeNil)
	sut.initialQueue()
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueue, err := ioutil.ReadDir(sut.syncQueue)
	if err != nil {
		t.Fatalf("failed to list files in syncQueue")
	}
	// Wait 100 ms so both can have the chance to queue, but the scheduled syncer doesn't start uploading yet.
	time.Sleep(time.Millisecond * 100)
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, len(filesInQueue), test.ShouldEqual, 2)

	sut.Close()
}

// Validates that if a scheduled syncer is killed after enqueueing a file, a new syncer will still pick it up and upload it.
// This is to simulate the case where a robot is killed mid-sync; we still want that sync to resume and finish when it
// turns back on.
func TestRecoversAfterKilledScheduled(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Put a file in syncDir; this simulates a file that was enqueued by some previous syncer.
	file1, _ := ioutil.TempFile(sut.syncQueue, "file1")
	defer os.Remove(file1.Name())

	// Put a file in captureDir; this simulates a file that was written but not yet queued by some previous syncer.
	// It should be synced even if it is not specified in the list passed to Queue.
	file2, _ := ioutil.TempFile(sut.captureDir, "file1")
	defer os.Remove(file2.Name())

	// Queue/upload with the syncer, let it run for a second.
	sut.initialQueue()
	err := sut.Queue([]string{})
	test.That(t, err, test.ShouldBeNil)
	err = sut.Upload()
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(time.Second)

	// Verify enqueued files were uploaded.
	filesInQueue, err := ioutil.ReadDir(sut.syncQueue)
	if err != nil {
		t.Fatalf("failed to list files in syncDir")
	}
	// Verify previously captured but not queued files were uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in syncDir")
	}
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)
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
	file1, _ := ioutil.TempFile(sut.captureDir, "file1")
	defer os.Remove(file1.Name())

	// Queue/upload with the syncer, let it run for a second.
	initialWaitTime = time.Millisecond * 25
	maxRetryInterval = time.Millisecond * 150
	sut.initialQueue()
	err := sut.Upload()
	test.That(t, err, test.ShouldBeNil)
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
