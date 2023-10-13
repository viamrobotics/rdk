// Package datasync contains interfaces for syncing data from robots to the app.viam.com cloud.
package datasync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

var (
	// InitialWaitTimeMillis defines the time to wait on the first retried upload attempt.
	InitialWaitTimeMillis = atomic.NewInt32(1000)
	// RetryExponentialFactor defines the factor by which the retry wait time increases.
	RetryExponentialFactor = atomic.NewInt32(2)
	maxRetryInterval       = time.Hour
)

// Manager is responsible for enqueuing files in captureDir and uploading them to the cloud.
type Manager interface {
	SyncFile(path string)
	SetArbitraryFileTags(tags []string)
	Close()
}

// syncer is responsible for uploading files in captureDir to the cloud.
type syncer struct {
	partID            string
	client            v1.DataSyncServiceClient
	logger            golog.Logger
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
	arbitraryFileTags []string

	progressLock sync.Mutex
	inProgress   map[string]bool

	syncErrs   chan error
	closed     atomic.Bool
	logRoutine sync.WaitGroup

	corruptedFilesDir string
}

// ManagerConstructor is a function for building a Manager.
type ManagerConstructor func(identity string, client v1.DataSyncServiceClient, logger golog.Logger, corruptedFilesDir string) (Manager, error)

// NewManager returns a new syncer.
func NewManager(
	identity string, client v1.DataSyncServiceClient, logger golog.Logger, corruptedFilesDir string) (Manager, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	ret := syncer{
		partID:            identity,
		client:            client,
		logger:            logger,
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
		arbitraryFileTags: []string{},
		inProgress:        make(map[string]bool),
		syncErrs:          make(chan error, 10),
		corruptedFilesDir: corruptedFilesDir,
	}
	ret.logRoutine.Add(1)
	goutils.PanicCapturingGo(func() {
		defer ret.logRoutine.Done()
		ret.logSyncErrs()
	})
	return &ret, nil
}

// Close closes all resources (goroutines) associated with s.
func (s *syncer) Close() {
	s.closed.Store(true)
	s.cancelFunc()
	s.backgroundWorkers.Wait()
	close(s.syncErrs)
	s.logRoutine.Wait()
	//nolint:errcheck
	_ = s.logger.Sync()
}

func (s *syncer) SetArbitraryFileTags(tags []string) {
	s.arbitraryFileTags = tags
}

func (s *syncer) SyncFile(path string) {
	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer s.backgroundWorkers.Done()
		select {
		case <-s.cancelCtx.Done():
			return
		default:
			if !s.markInProgress(path) {
				return
			}
			defer s.unmarkInProgress(path)
			//nolint:gosec
			f, err := os.Open(path)
			if err != nil {
				// Don't log if the file does not exist, because that means it was successfully synced and deleted
				// in between paths being built and this executing.
				if !errors.Is(err, os.ErrNotExist) {
					s.logger.Errorw("error opening file", "error", err)
				}
				return
			}

			if datacapture.IsDataCaptureFile(f) {
				captureFile, err := datacapture.ReadFile(f)
				if err != nil {
					newPath := filepath.Join(s.corruptedFilesDir, f.Name())
					s.syncErrs <- errors.Wrapf(err, "error reading data capture file. moving to %s", newPath)
					err := f.Close()
					if err != nil {
						s.syncErrs <- errors.Wrap(err, "error closing data capture file")
					}
					if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
						s.syncErrs <- errors.Wrapf(err, "could not create target directory path %s", filepath.Dir(newPath))
					}
					if err := os.Rename(f.Name(), newPath); err != nil {
						s.syncErrs <- errors.Wrap(err, "error moving corrupted data capture file")
					}
					return
				}
				s.syncDataCaptureFile(captureFile)
			} else {
				s.syncArbitraryFile(f)
			}
		}
	})
}

func (s *syncer) syncDataCaptureFile(f *datacapture.File) {
	uploadErr := exponentialRetry(
		s.cancelCtx,
		func(ctx context.Context) error {
			err := uploadDataCaptureFile(ctx, s.client, f, s.partID)
			if err != nil {
				s.syncErrs <- errors.Wrap(err, fmt.Sprintf("error uploading file %s", f.GetPath()))
			}
			return err
		},
	)
	if uploadErr != nil {
		err := f.Close()
		if err != nil {
			s.syncErrs <- errors.Wrap(err, "error closing data capture file")
		}

		errStatus := status.Convert(uploadErr)
		if errStatus.Code() == codes.InvalidArgument {
			newPath := filepath.Join(s.corruptedFilesDir, f.GetPath())
			if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
				s.syncErrs <- errors.Wrapf(err, "could not create target directory path %s", filepath.Dir(newPath))
			}
			if err := os.Rename(f.GetPath(), newPath); err != nil {
				s.syncErrs <- errors.Wrap(err, "error moving corrupted data capture file")
			}
		}
		return
	}
	if err := f.Delete(); err != nil {
		s.syncErrs <- errors.Wrap(err, "error deleting data capture file")
		return
	}
}

func (s *syncer) syncArbitraryFile(f *os.File) {
	uploadErr := exponentialRetry(
		s.cancelCtx,
		func(ctx context.Context) error {
			uploadErr := uploadArbitraryFile(ctx, s.client, f, s.partID, s.arbitraryFileTags)
			if uploadErr != nil {
				s.syncErrs <- errors.Wrap(uploadErr, fmt.Sprintf("error uploading file %s", f.Name()))
			}

			errStatus := status.Convert(uploadErr)
			if errStatus.Code() == codes.InvalidArgument {
				newPath := filepath.Join(s.corruptedFilesDir, f.Name())
				if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
					s.syncErrs <- errors.Wrapf(err, "could not create target directory path %s", filepath.Dir(newPath))
				}
				if err := os.Rename(f.Name(), newPath); err != nil {
					s.syncErrs <- errors.Wrap(err, "error moving corrupted data capture file")
				}
			}
			return uploadErr
		})
	if uploadErr != nil {
		err := f.Close()
		if err != nil {
			s.syncErrs <- errors.Wrap(err, "error closing data capture file")
		}
		return
	}
	if err := os.Remove(f.Name()); err != nil {
		s.syncErrs <- errors.Wrap(err, fmt.Sprintf("error deleting file %s", f.Name()))
		return
	}
}

// markInProgress marks path as in progress in s.inProgress. It returns true if it changed the progress status,
// or false if the path was already in progress.
func (s *syncer) markInProgress(path string) bool {
	s.progressLock.Lock()
	defer s.progressLock.Unlock()
	if s.inProgress[path] {
		return false
	}
	s.inProgress[path] = true
	return true
}

func (s *syncer) unmarkInProgress(path string) {
	s.progressLock.Lock()
	defer s.progressLock.Unlock()
	delete(s.inProgress, path)
}

func (s *syncer) logSyncErrs() {
	for err := range s.syncErrs {
		if s.closed.Load() {
			// Don't log context cancellation errors if the Manager has already been closed. This means the Manager
			// cancelled the context, and the context cancellation error is expected.
			if strings.Contains(err.Error(), context.Canceled.Error()) {
				continue
			}
		}
		s.logger.Error(err)
	}
}

// exponentialRetry calls fn and retries with exponentially increasing waits from initialWait to a
// maximum of maxRetryInterval.
func exponentialRetry(cancelCtx context.Context, fn func(cancelCtx context.Context) error) error {
	// Only create a ticker and enter the retry loop if we actually need to retry.
	var err error
	if err = fn(cancelCtx); err == nil {
		return nil
	}
	// Don't retry non-retryable errors.
	s := status.Convert(err)
	if s.Code() == codes.InvalidArgument {
		return err
	}

	// First call failed, so begin exponentialRetry with a factor of RetryExponentialFactor
	nextWait := time.Millisecond * time.Duration(InitialWaitTimeMillis.Load())
	ticker := time.NewTicker(nextWait)
	for {
		if err := cancelCtx.Err(); err != nil {
			return err
		}
		select {
		// If cancelled, return nil.
		case <-cancelCtx.Done():
			ticker.Stop()
			return cancelCtx.Err()
			// Otherwise, try again after nextWait.
		case <-ticker.C:
			if err := fn(cancelCtx); err != nil {
				// If error, retry with a new nextWait.
				ticker.Stop()
				nextWait = getNextWait(nextWait)
				ticker = time.NewTicker(nextWait)
				continue
			}
			// If no error, return.
			ticker.Stop()
			return nil
		}
	}
}

func getNextWait(lastWait time.Duration) time.Duration {
	if lastWait == time.Duration(0) {
		return time.Millisecond * time.Duration(InitialWaitTimeMillis.Load())
	}
	nextWait := lastWait * time.Duration(RetryExponentialFactor.Load())
	if nextWait > maxRetryInterval {
		return maxRetryInterval
	}
	return nextWait
}
