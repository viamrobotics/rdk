package datamanager

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"
)

var (
	initialWaitTime        = time.Second
	retryExponentialFactor = 2
	maxRetryInterval       = time.Hour
)

// syncManager is responsible for uploading files to the cloud every syncInterval.
type syncManager interface {
	Start()
	Sync(paths []string)
	Close()
}

// syncer is responsible for enqueuing files in captureDir and uploading them to the cloud.
type syncer struct {
	captureDir        string
	logger            golog.Logger
	progressTracker   progressTracker
	uploadFn          func(ctx context.Context, path string) error
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

// newSyncer returns a new syncer.
func newSyncer(logger golog.Logger, captureDir string) *syncer {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	ret := syncer{
		logger:     logger,
		captureDir: captureDir,
		progressTracker: progressTracker{
			lock: &sync.Mutex{},
			m:    make(map[string]struct{}),
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

// Start queues any files already in captureDir that haven't been modified in s.queueWaitTime time, and kicks off a
// goroutine to constantly upload files in the queue.
func (s *syncer) Start() {
	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer s.backgroundWorkers.Done()
		if err := filepath.WalkDir(s.captureDir, s.uploadWalkDirFunc); err != nil {
			s.logger.Errorf("failed to upload queued file: %v", err)
		}
	})
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
func (s *syncer) uploadWalkDirFunc(path string, di fs.DirEntry, err error) error {
	if err != nil {
		s.logger.Errorw("failed to upload queued file", "error", err)

		return nil
	}

	if di.IsDir() {
		return nil
	}

	s.upload(s.cancelCtx, path)
	return nil
}

func (s *syncer) upload(ctx context.Context, path string) {
	if s.progressTracker.inProgress(path) {
		return
	}

	s.progressTracker.mark(path)
	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer s.backgroundWorkers.Done()
		exponentialRetry(
			ctx,
			func(ctx context.Context) error { return s.uploadFn(ctx, path) },
			s.logger,
		)
	})
	// TODO: If upload completed successfully, unmark in-progress and delete file.
}

func (s *syncer) Sync(paths []string) {
	for _, path := range paths {
		s.upload(s.cancelCtx, path)
	}
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

//nolint:unused
func (p *progressTracker) unmark(k string) {
	p.lock.Lock()
	delete(p.m, k)
	p.lock.Unlock()
}

// exponentialRetry calls fn, logs any errors, and retries with exponentially increasing waits from initialWait to a
// maximum of maxRetryInterval.
func exponentialRetry(ctx context.Context, fn func(ctx context.Context) error, log golog.Logger) {
	// Only create a ticker and enter the retry loop if we actually need to retry.
	if err := fn(ctx); err == nil {
		return
	}

	// First call failed, so begin exponentialRetry with a factor of retryExponentialFactor
	nextWait := initialWaitTime
	ticker := time.NewTicker(nextWait)
	for {
		if err := ctx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Errorw("context closed unexpectedly", "error", err)
			}
			return
		}

		select {
		// If cancelled, return nil.
		case <-ctx.Done():
			ticker.Stop()
			return
		// Otherwise, try again after nextWait.
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				// If error, retry with a new nextWait.
				log.Errorw("error while uploading file", "error", err)
				ticker.Stop()
				nextWait = getNextWait(nextWait)
				ticker = time.NewTicker(nextWait)
				continue
			}
			// If no error, return.
			ticker.Stop()
			return
		}
	}
}

func getNextWait(lastWait time.Duration) time.Duration {
	if lastWait == time.Duration(0) {
		return initialWaitTime
	}
	nextWait := lastWait * time.Duration(retryExponentialFactor)
	if nextWait > maxRetryInterval {
		return maxRetryInterval
	}
	return nextWait
}
