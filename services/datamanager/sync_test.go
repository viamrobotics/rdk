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
	"go.viam.com/test"
)

func newTestSyncer(captureDir string, queue string, logger golog.Logger, uploadCount *uint64) syncer {
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	return syncer{
		captureDir:     captureDir,
		syncQueue:      queue,
		logger:         logger,
		queueWaitTime:  time.Nanosecond,
		inProgress:     map[string]struct{}{},
		inProgressLock: &sync.Mutex{},
		uploadFn: func(ctx context.Context, path string) error {
			atomic.AddUint64(uploadCount, 1)
			_ = os.Remove(path)
			return nil
		},
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFn,
	}
}

// Validates that for some captureDir, files are enqueued and uploaded exactly once.
func TestQueuesAndUploadsOnce(t *testing.T) {
	captureDir := t.TempDir()
	syncDir := t.TempDir()
	l := golog.NewTestLogger(t)

	var uploadCount uint64
	sut := newTestSyncer(captureDir, syncDir, l, &uploadCount)

	// Start syncer.
	sut.Start()

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(captureDir, "whatever2")
	defer os.Remove(file2.Name())
	err := sut.Enqueue([]string{file1.Name(), file2.Name()})
	test.That(t, err, test.ShouldBeNil)
	// Give it a second to run and upload files.
	time.Sleep(time.Second)

	// Verify files were enqueued and uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueue, err := ioutil.ReadDir(syncDir)
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
	captureDir := t.TempDir()
	syncDir := t.TempDir()
	l := golog.NewTestLogger(t)

	var uploadCount uint64
	sut := newTestSyncer(captureDir, syncDir, l, &uploadCount)

	// Put a file in syncDir; this simulates a file that was enqueued by some previous syncer.
	file1, _ := ioutil.TempFile(syncDir, "whatever")
	defer os.Remove(file1.Name())

	// Put a file in captureDir; this simulates a file that was written but not yet queued by some previous syncer.
	// It should be synced even if it is not specified in the list passed to Enqueue.
	file2, _ := ioutil.TempFile(captureDir, "whatever")
	defer os.Remove(file2.Name())

	// Start syncer, let it run for a second.
	sut.Start()
	err := sut.Enqueue([]string{})
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(time.Second)

	// Verify enqueued files were uploaded.
	filesInQueue, err := ioutil.ReadDir(syncDir)
	if err != nil {
		t.Fatalf("failed to list files in syncDir")
	}
	// Verify previously captured but not queued files were uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in syncDir")
	}
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}
