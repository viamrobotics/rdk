// Package datasync contains interfaces for syncing data from robots to the app.viam.com cloud.
package datasync

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/atomic"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager/datacapture"
)

var (
	// InitialWaitTimeMillis defines the time to wait on the first retried upload attempt.
	InitialWaitTimeMillis = atomic.NewInt32(1000)
	// RetryExponentialFactor defines the factor by which the retry wait time increases.
	RetryExponentialFactor = atomic.NewInt32(2)
	// OfflineWaitTimeSeconds defines the amount of time to wait to retry if the machine is offline.
	OfflineWaitTimeSeconds = atomic.NewInt32(60)
	maxRetryInterval       = 24 * time.Hour
)

// FailedDir is a subdirectory of the capture directory that holds any files that could not be synced.
const FailedDir = "failed"

// MaxParallelSyncRoutines is the maximum number of sync goroutines that can be running at once.
const MaxParallelSyncRoutines = 1000

// Manager is responsible for enqueuing files in captureDir and uploading them to the cloud.
type Manager interface {
	SendFileToSync(path string)
	SyncFile(path string)
	SetArbitraryFileTags(tags []string)
	Close()
	MarkInProgress(path string) bool
	UnmarkInProgress(path string)
}

// syncer is responsible for uploading files in captureDir to the cloud.
type syncer struct {
	partID            string
	client            v1.DataSyncServiceClient
	logger            logging.Logger
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
	arbitraryFileTags []string

	progressLock sync.Mutex
	inProgress   map[string]bool

	syncErrs   chan error
	closed     atomic.Bool
	logRoutine sync.WaitGroup

	filesToSync chan string

	captureDir string
}

// ManagerConstructor is a function for building a Manager.
type ManagerConstructor func(identity string, client v1.DataSyncServiceClient, logger logging.Logger,
	captureDir string, maxSyncThreadsConfig int, filesToSync chan string) Manager

// NewManager returns a new syncer.
func NewManager(identity string, client v1.DataSyncServiceClient, logger logging.Logger,
	captureDir string, maxSyncThreads int, filesToSync chan string,
) Manager {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	logger.Infof("Making new syncer with %d max threads", maxSyncThreads)
	ret := syncer{
		partID:            identity,
		client:            client,
		logger:            logger,
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
		arbitraryFileTags: []string{},
		inProgress:        make(map[string]bool),
		syncErrs:          make(chan error, 10),
		filesToSync:       filesToSync,
		captureDir:        captureDir,
	}
	ret.logRoutine.Add(1)
	goutils.PanicCapturingGo(func() {
		defer ret.logRoutine.Done()
		ret.logSyncErrs()
	})

	for i := 0; i < maxSyncThreads; i++ {
		ret.backgroundWorkers.Add(1)
		go func() {
			defer ret.backgroundWorkers.Done()
			for {
				if cancelCtx.Err() != nil {
					return
				}
				select {
				case <-cancelCtx.Done():
					return
				case path, ok := <-ret.filesToSync:
					if !ok {
						return
					}
					ret.SyncFile(path)
				}
			}
		}()
	}

	return &ret
}

// Close closes all resources (goroutines) associated with s.
func (s *syncer) Close() {
	s.closed.Store(true)
	s.cancelFunc()
	s.backgroundWorkers.Wait()
	close(s.syncErrs)
	s.logRoutine.Wait()
}

func (s *syncer) SetArbitraryFileTags(tags []string) {
	s.arbitraryFileTags = tags
}

func (s *syncer) SendFileToSync(path string) {
	select {
	case s.filesToSync <- path:
		return
	case <-s.cancelCtx.Done():
		return
	}
}

func (s *syncer) SyncFile(path string) {
	// If the file is already being synced, do not kick off a new goroutine.
	// The goroutine will again check and return early if sync is already in progress.
	if !s.MarkInProgress(path) {
		return
	}
	defer s.UnmarkInProgress(path)
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
			if err = f.Close(); err != nil {
				s.syncErrs <- errors.Wrap(err, "error closing data capture file")
			}
			if err := moveFailedData(f.Name(), s.captureDir); err != nil {
				s.syncErrs <- errors.Wrap(err, fmt.Sprintf("error moving corrupted data %s", f.Name()))
			}
			return
		}
		s.syncDataCaptureFile(captureFile)
	} else {
		s.syncArbitraryFile(f)
	}
}

func (s *syncer) syncDataCaptureFile(f *datacapture.File) {
	uploadErr := exponentialRetry(
		s.cancelCtx,
		func(ctx context.Context) error {
			err := uploadDataCaptureFile(ctx, s.client, f, s.partID)
			if err != nil {
				s.syncErrs <- errors.Wrap(err, fmt.Sprintf("error uploading file %s, size: %d, md: %s",
					f.GetPath(), f.Size(), f.ReadMetadata()))
			}
			return err
		},
	)
	if uploadErr != nil {
		err := f.Close()
		if err != nil {
			s.syncErrs <- errors.Wrap(err, "error closing data capture file")
		}

		if !isRetryableGRPCError(uploadErr) {
			if err := moveFailedData(f.GetPath(), s.captureDir); err != nil {
				s.syncErrs <- errors.Wrap(err, fmt.Sprintf("error moving corrupted data %s", f.GetPath()))
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

			if !isRetryableGRPCError(uploadErr) {
				if err := moveFailedData(f.Name(), path.Dir(f.Name())); err != nil {
					s.syncErrs <- errors.Wrap(err, fmt.Sprintf("error moving corrupted data %s", f.Name()))
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

// MarkInProgress marks path as in progress in s.inProgress. It returns true if it changed the progress status,
// or false if the path was already in progress.
func (s *syncer) MarkInProgress(path string) bool {
	s.progressLock.Lock()
	defer s.progressLock.Unlock()
	if s.inProgress[path] {
		s.logger.Debugw("File already in progress, trying to mark it again", "file", path)
		return false
	}
	s.inProgress[path] = true
	return true
}

// UnmarkInProgress unmarks a path as in progress in s.inProgress.
func (s *syncer) UnmarkInProgress(path string) {
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
	if !isRetryableGRPCError(err) {
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
				nextWait = getNextWait(nextWait, isOfflineGRPCError(err))
				ticker = time.NewTicker(nextWait)
				continue
			}
			// If no error, return.
			ticker.Stop()
			return nil
		}
	}
}

func isOfflineGRPCError(err error) bool {
	errStatus := status.Convert(err)
	return errStatus.Code() == codes.Unavailable
}

// isRetryableGRPCError returns true if we should retry syncing and otherwise
// returns false so that the data gets moved to the corrupted data directory.
func isRetryableGRPCError(err error) bool {
	errStatus := status.Convert(err)
	return errStatus.Code() != codes.InvalidArgument && !errors.Is(err, proto.Error)
}

// moveFailedData takes any data that could not be synced in the parentDir and
// moves it to a new subdirectory "failed" that will not be synced.
func moveFailedData(path, parentDir string) error {
	// Remove the parentDir part of the path to the corrupted data
	relativePath, err := filepath.Rel(parentDir, path)
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("error getting relative path of corrupted data: %s", path))
	}
	// Create a new directory parentDir/corrupted/pathToFile
	newDir := filepath.Join(parentDir, FailedDir, filepath.Dir(relativePath))
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return errors.Wrapf(err, fmt.Sprintf("error making new directory for corrupted data: %s", path))
	}
	// Move the file from parentDir/pathToFile/file.ext to parentDir/corrupted/pathToFile/file.ext
	newPath := filepath.Join(newDir, filepath.Base(path))
	if err := os.Rename(path, newPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Wrapf(err, fmt.Sprintf("error moving corrupted data: %s", path))
		}
	}
	return nil
}

func getNextWait(lastWait time.Duration, isOffline bool) time.Duration {
	if lastWait == time.Duration(0) {
		return time.Millisecond * time.Duration(InitialWaitTimeMillis.Load())
	}

	if isOffline {
		return time.Second * time.Duration(OfflineWaitTimeSeconds.Load())
	}

	nextWait := lastWait * time.Duration(RetryExponentialFactor.Load())
	if nextWait > maxRetryInterval {
		return maxRetryInterval
	}
	return nextWait
}
