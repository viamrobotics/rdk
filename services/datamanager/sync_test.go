package datamanager

import (
	"context"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func newTestSyncer(t *testing.T, uploadFn func(ctx context.Context, path string) error) syncer {
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	captureDir := t.TempDir()
	syncQueue := t.TempDir()
	l := golog.NewTestLogger(t)

	return syncer{
		captureDir:     captureDir,
		syncQueue:      syncQueue,
		logger:         l,
		queueWaitTime:  time.Nanosecond,
		inProgress:     map[string]struct{}{},
		inProgressLock: &sync.Mutex{},
		uploadFn:       uploadFn,
		cancelCtx:      cancelCtx,
		cancelFunc:     cancelFn,
	}
}

// Validates that for some captureDir, files are enqueued and uploaded exactly once.
func TestQueuesAndUploadsOnce(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Start syncer.
	sut.Start()

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(sut.captureDir, "whatever2")
	defer os.Remove(file2.Name())
	err := sut.Enqueue([]string{file1.Name(), file2.Name()})
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
		t.Fatalf("failed to list files in captureDir")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)
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
	file1, _ := ioutil.TempFile(sut.syncQueue, "whatever")
	defer os.Remove(file1.Name())

	// Put a file in captureDir; this simulates a file that was written but not yet queued by some previous syncer.
	// It should be synced even if it is not specified in the list passed to Enqueue.
	file2, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file2.Name())

	// Start syncer, let it run for a second.
	sut.Start()
	err := sut.Enqueue([]string{})
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

// TODO: test that exponential increase is working
// TODO: test that max wait time is respected
func TestRetriesUploads(t *testing.T) {
	// Validate that a failed upload is retried.
	failureCount := 0
	successCount := 0
	uploadFunc := func(ctx context.Context, path string) error {
		if failureCount >= 3 {
			successCount++
			return nil
		}
		failureCount++
		return errors.New("fail for the first 3 tries, then succeed")
	}
	sut := newTestSyncer(t, uploadFunc)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file1.Name())

	// Start syncer, let it run for a second.
	retryExponentialFactor = 1
	initialWaitTime = time.Millisecond
	sut.Start()
	err := sut.Enqueue([]string{})
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(time.Second)

	// Test that it was successfully uploaded, and upload failed 3 times and succeeded once.
	test.That(t, failureCount, test.ShouldEqual, 3)
	test.That(t, successCount, test.ShouldEqual, 1)
	sut.Close()
}
