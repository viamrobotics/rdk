package datamanager

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"
)

// syncManager is responsible for uploading files to the cloud every syncInterval.
type syncManager interface {
	Start()
	Enqueue(filesToQueue []string) error
	Close()
}

// syncer is responsible for enqueuing files in captureDir and uploading them to the cloud.
type syncer struct {
	captureDir        string
	syncQueue         string
	logger            golog.Logger
	queueWaitTime     time.Duration
	progressTracker   progressTracker
	uploadFn          func(ctx context.Context, path string) error
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

// newSyncer returns a new syncer.
func newSyncer(queuePath string, logger golog.Logger, captureDir string) *syncer {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	ret := syncer{
		syncQueue:     queuePath,
		logger:        logger,
		captureDir:    captureDir,
		queueWaitTime: time.Minute,
		progressTracker: progressTracker{
			lock: &sync.Mutex{},
			m:    make(map[string]bool),
		},
		backgroundWorkers: sync.WaitGroup{},
		uploadFn: func(ctx context.Context, path string) error {
			return nil
		},
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	return &ret
}

// Enqueue moves files that are no longer being written to from captureDir to SyncQueuePath.
func (s *syncer) Enqueue(filesToQueue []string) error {
	for _, filePath := range filesToQueue {
		subPath, err := s.getPathUnderCaptureDir(filePath)
		if err != nil {
			return errors.Errorf("could not get path under capture directory: %v", err)
		}

		if err := os.MkdirAll(filepath.Dir(path.Join(s.syncQueue, subPath)), 0o700); err != nil {
			return errors.Errorf("failed create directories under sync enqueue: %v", err)
		}

		if err := os.Rename(filePath, path.Join(s.syncQueue, subPath)); err != nil {
			return errors.Errorf("failed to move file to sync enqueue: %v", err)
		}
	}
	return nil
}

// Start queues any files already in captureDir that haven't been modified in s.queueWaitTime time, and kicks off a
// goroutine to constantly upload files in the queue.
func (s *syncer) Start() {
	// First, move any files in captureDir to queue.
	if err := filepath.WalkDir(s.captureDir, s.queueFile); err != nil {
		s.logger.Errorf("failed to move files to sync queue: %v", err)
	}

	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		ticker := time.NewTicker(time.Millisecond * 500)
		defer ticker.Stop()
		defer s.backgroundWorkers.Done()
		for {
			if err := s.cancelCtx.Err(); err != nil {
				if !errors.Is(err, context.Canceled) {
					s.logger.Errorw("sync context closed unexpectedly", "error", err)
				}
				return
			}
			select {
			case <-s.cancelCtx.Done():
				return
			case <-ticker.C:
				if err := filepath.WalkDir(s.syncQueue, s.upload); err != nil {
					s.logger.Errorf("failed to upload queued file: %v", err)
				}
			}
		}
	})
}

// queueFile is an fs.WalkDirFunc that moves matching files to s.syncQueue.
func (s *syncer) queueFile(filePath string, di fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if di.IsDir() {
		return nil
	}

	fileInfo, err := di.Info()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to get file info for filepath %s", filePath))
	}

	// If it's been written to in the last s.queueWaitTime, it's still active and shouldn't be queued.
	if time.Since(fileInfo.ModTime()) < s.queueWaitTime {
		return nil
	}

	subPath, err := s.getPathUnderCaptureDir(filePath)
	if err != nil {
		return errors.Wrap(err, "could not get path under capture directory")
	}

	if err = os.MkdirAll(filepath.Dir(path.Join(s.syncQueue, subPath)), 0o700); err != nil {
		return errors.Wrap(err, "failed create directories under sync enqueue")
	}

	if err := os.Rename(filePath, path.Join(s.syncQueue, subPath)); err != nil {
		return errors.Wrap(err, "failed to move file to sync enqueue")
	}
	return nil
}

func (s *syncer) getPathUnderCaptureDir(filePath string) (string, error) {
	if idx := strings.Index(filePath, s.captureDir); idx != -1 {
		return filePath[idx+len(s.captureDir):], nil
	}
	return "", errors.Errorf("file path %s is not under capture directory %s", filePath, s.captureDir)
}

// Close closes all resources (goroutines) associated with s.
func (s *syncer) Close() {
	s.cancelFunc()
	s.backgroundWorkers.Wait()
}

// upload is an fs.WalkDirFunc that uploads files to Viam cloud storage.
func (s *syncer) upload(path string, di fs.DirEntry, err error) error {
	if err != nil {
		s.logger.Errorw("failed to upload queued file", "error", err)
		// nolint
		return nil
	}

	if di.IsDir() {
		return nil
	}

	if s.progressTracker.inProgress(path) {
		return nil
	}

	s.progressTracker.mark(path)
	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer s.backgroundWorkers.Done()
		err = s.uploadFn(s.cancelCtx, path)
		if err != nil {
			s.progressTracker.unmark(path)
			s.logger.Errorf("failed to upload queued file: %v", err)
		}
	})
	// TODO: If upload completed successfully, unmark in-progress and delete file.
	return nil
}

type progressTracker struct {
	lock *sync.Mutex
	m    map[string]struct{}
}

func (p *progressTracker) inProgress(k string) bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, ok := p.m[k]
	return ok
}

func (p *progressTracker) mark(k string) {
	p.lock.Lock()
	p.m[k] = struct{}{}
	p.lock.Unlock()
}

func (p *progressTracker) unmark(k string) {
	p.lock.Lock()
	delete(p.m, k)
	p.lock.Unlock()
}
