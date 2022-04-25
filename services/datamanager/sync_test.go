package datamanager

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func newTestSyncer(captureDir string, queue string, logger golog.Logger) syncer {
	return syncer{
		captureDir:    captureDir,
		syncQueue:     queue,
		logger:        logger,
		queueWaitTime: time.Millisecond,
		cancelCtx:     context.Background(),
		cancelFunc:    func() {},
	}
}

// Validates that for some captureDir, a file is enqueued and uploaded exactly once.
func TestQueuesAndUploadsOnce(t *testing.T) {
	captureDir := t.TempDir()
	syncDir := t.TempDir()
	l := golog.NewTestLogger(t)

	sut := newTestSyncer(captureDir, syncDir, l)
	uploadCount := 0
	sut.uploader = uploader{
		inProgress: map[string]struct{}{},
		uploadFn: func(path string) error {
			uploadCount++
			_ = os.Remove(path)
			return nil
		},
	}

	// Put a file in captureDir.
	file1, _ := ioutil.TempFile(captureDir, "whatever")
	defer os.Remove(file1.Name())

	// Start syncer, let it run for a couple seconds.
	go sut.Enqueue(time.Millisecond)
	go sut.UploadSynced()
	time.Sleep(time.Second * 2)

	// Verify file was enqueued and uploaded (moved from captureDir to syncDir).
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
	test.That(t, uploadCount, test.ShouldEqual, 1)
}

// Validates that if a syncer is killed after enqueing a file, a new syncer will still pick it up and upload it.
// This is to simulate the case where a robot is killed mid-sync; we still want that sync to resume and finish when it
// turns back on.
func TestRecoversAfterKilled(t *testing.T) {
	captureDir := t.TempDir()
	syncDir := t.TempDir()
	l := golog.NewTestLogger(t)

	sut := newTestSyncer(captureDir, syncDir, l)
	uploadCount := 0
	sut.uploader = uploader{
		inProgress: map[string]struct{}{},
		uploadFn: func(path string) error {
			uploadCount++
			_ = os.Remove(path)
			return nil
		},
	}

	// Put a file in syncDir; this simulates a file that was enqueued by some previous syncer.
	file1, _ := ioutil.TempFile(syncDir, "whatever")
	defer os.Remove(file1.Name())

	// Start syncer, let it run for a couple seconds.
	go sut.Enqueue(time.Millisecond * 250)
	go sut.UploadSynced()
	time.Sleep(time.Second * 2)

	// Verify enqueued file was uploaded.
	filesInQueue, err := ioutil.ReadDir(syncDir)
	if err != nil {
		t.Fatalf("failed to list files in syncDir")
	}
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)
	test.That(t, uploadCount, test.ShouldEqual, 1)
}
