package sync

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/utils"
)

var (
	// InitialWaitTimeMillis defines the time to wait on the first retried upload attempt.
	initialWaitTimeMillis = 200
	// RetryExponentialFactor defines the factor by which the retry wait time increases.
	retryExponentialFactor = 2
	// OfflineWaitTimeSeconds defines the amount of time to wait to retry if the machine is offline.
	offlineWaitTimeSeconds = 60
	maxRetryInterval       = time.Hour
)

// Syncer is responsible for uploading files in captureDir to the cloud.
type Syncer struct {
	partID            string
	client            v1.DataSyncServiceClient
	cloudConn         rpc.ClientConn
	logger            logging.Logger
	workersWg         sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
	arbitraryFileTags []string
	clock             clk.Clock
	flushCollectors   func()
	syncSensor        sensor.Sensor

	progressLock sync.Mutex
	inProgress   map[string]bool

	filesToSync chan string

	captureDir             string
	fileLastModifiedMillis int
	syncIntervalMins       float64
	syncPaths              []string

	schedulerWorkers utils.StoppableWorkers
}

// SyncerConstructor is a function for building a Manager.
type SyncerConstructor func(
	identity string,
	client v1.DataSyncServiceClient,
	logger logging.Logger,
	captureDir string,
	maxSyncThreadsConfig int,
	filesToSync chan string) Syncer

// NewSyncer returns a new syncer.
func NewSyncer(
	configWithDeps configWithDeps,
	partID string,
	client v1.DataSyncServiceClient,
	conn rpc.ClientConn,
	clock clk.Clock,
	flushCollectors func(),
	schedule bool,
	logger logging.Logger,
) *Syncer {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	logger.Infof("Making new syncer with %d max threads", configWithDeps.config.MaximumNumSyncThreads)
	s := Syncer{
		cancelCtx:              cancelCtx,
		cancelFunc:             cancelFunc,
		partID:                 partID,
		clock:                  clock,
		client:                 client,
		logger:                 logger,
		arbitraryFileTags:      configWithDeps.config.Tags,
		inProgress:             make(map[string]bool),
		filesToSync:            make(chan string),
		captureDir:             configWithDeps.config.CaptureDir,
		fileLastModifiedMillis: configWithDeps.config.FileLastModifiedMillis,
		flushCollectors:        flushCollectors,
		syncIntervalMins:       configWithDeps.config.SyncIntervalMins,
		syncSensor:             configWithDeps.syncSensor,
		syncPaths:              configWithDeps.config.syncPaths(),
	}

	for i := 0; i < configWithDeps.config.MaximumNumSyncThreads; i++ {
		s.workersWg.Add(1)
		goutils.ManagedGo(s.startWorker, s.workersWg.Done)
	}

	if schedule {
		s.schedulerWorkers = utils.NewStoppableWorkers(s.maybeSyncOnInterval)
	} else {
		s.schedulerWorkers = utils.NewStoppableWorkers()
	}

	return &s
}

func (s *Syncer) startWorker() {
	for {
		if s.cancelCtx.Err() != nil {
			return
		}
		select {
		case <-s.cancelCtx.Done():
			return
		case path := <-s.filesToSync:
			s.syncFile(path)
		}
	}
}

// Close closes all resources (goroutines) associated with s.
func (s *Syncer) Close() {
	s.cancelFunc()
	if s.cloudConn != nil {
		goutils.UncheckedError(s.cloudConn.Close())
	}
	s.schedulerWorkers.Stop()
	s.workersWg.Wait()
}

func (s *Syncer) SendFileToSync(ctx context.Context, path string) {
	// s.logger.Infof("SendFileToSync(%s)", path)
	select {
	case <-ctx.Done():
		return
	case <-s.cancelCtx.Done():
		return
	case s.filesToSync <- path:
		return
	}
}

func (s *Syncer) maybeSyncOnInterval(ctx context.Context) {
	// time.Duration loses precision at low floating point values, so turn intervalMins to milliseconds.
	intervalMillis := 60000.0 * s.syncIntervalMins
	// The ticker must be created before uploadData returns to prevent race conditions between clock.Ticker and
	// clock.Add in sync_test.go.
	interval := time.Millisecond * time.Duration(intervalMillis)

	tkr := s.clock.Ticker(interval)
	defer tkr.Stop()
	s.logger.Infof("maybeSyncOnInterval START %p", tkr)
	defer s.logger.Info("maybeSyncOnInterval END")

	for {
		if err := ctx.Err(); err != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-tkr.C:
			// TODO: how is this possible?
			// If selective sync is disabled, sync. If it is enabled, check the condition below.
			shouldSync := s.syncSensor == nil
			// If selective sync is enabled and the sensor has been properly initialized,
			// try to get the reading from the selective sensor that indicates whether to sync
			if s.syncSensor != nil {
				shouldSync = readyToSync(ctx, s.syncSensor, s.logger)
			}

			c, ok := s.cloudConn.(*rpc.GrpcOverHTTPClientConn)
			if !ok {
				s.logger.Error("can't turn s.cloudConn into a grpc.ClientConn")
				return
			}

			if online := c.ClientConn.GetState() == connectivity.Ready; online && shouldSync {
				s.flushAndSendFilesToSync(ctx)
			}
		}
	}
}

func (s *Syncer) flushAndSendFilesToSync(ctx context.Context) {
	// Retrieve all files in capture dir and send them to the syncer
	s.flushCollectors()
	s.getAllFilesToSync(ctx)
}

func (s *Syncer) getAllFilesToSync(ctx context.Context) {
	// syncer.logger.Info("getAllFilesToSync")
	for _, dir := range s.syncPaths {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if ctx.Err() != nil {
				s.logger.Info(path + " err: " + ctx.Err().Error())
				return filepath.SkipAll
			}
			if err != nil {
				s.logger.Info(path + " err: " + err.Error())
				//nolint:nilerr
				return nil
			}

			// Do not sync the files in the corrupted data directory.
			if info.IsDir() && info.Name() == FailedDir {
				return filepath.SkipDir
			}

			if info.IsDir() {
				return nil
			}
			// If a file was modified within the past lastModifiedMillis, do not sync it (data
			// may still be being written).
			timeSinceMod := s.clock.Since(info.ModTime())
			// When using a mock clock in tests, this can be negative since the file system will still use the system clock.
			// Take max(timeSinceMod, 0) to account for this.
			if timeSinceMod < 0 {
				timeSinceMod = 0
			}
			isCompletedCaptureFile := filepath.Ext(path) == datacapture.FileExt
			isNonCaptureFileThatIsNotBeingWrittenTo := filepath.Ext(path) != datacapture.InProgressFileExt &&
				filepath.Ext(path) != datacapture.FileExt &&
				timeSinceMod >= time.Duration(s.fileLastModifiedMillis)*time.Millisecond
			if isCompletedCaptureFile || isNonCaptureFileThatIsNotBeingWrittenTo && !s.InProgress(path) {
				s.SendFileToSync(ctx, path)
			}
			return nil
		})
		goutils.UncheckedError(err)
	}
}

func (s *Syncer) syncFile(path string) {
	// s.logger.Infof("syncFile: %s", path)
	// If the file is already being synced, do not kick off a new goroutine.
	// The goroutine will again check and return early if sync is already in progress.
	if !s.MarkInProgress(path) {
		// s.logger.Warnf("syncFile already in progress %s", path)
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
		s.logger.Infof("IsDataCaptureFile: %s", path)
		captureFile, err := datacapture.ReadFile(f)
		if err != nil {
			s.logger.Infof("IsDataCaptureFile: err: %s", err.Error())
			if err = f.Close(); err != nil {
				s.logger.Error(errors.Wrap(err, "error closing data capture file").Error())
			}
			if err := moveFailedData(f.Name(), s.captureDir); err != nil {
				s.logger.Error(err)
			}
			return
		}
		s.syncDataCaptureFile(captureFile)
	} else {
		s.logger.Infof("ArbitraryFile: %s", path)
		s.syncArbitraryFile(f)
	}
}

func (s *Syncer) syncDataCaptureFile(f *datacapture.File) {
	fun := func(ctx context.Context) error {
		errMetadata := fmt.Sprintf("error uploading data capture file %s, size: %d, md: %s", f.GetPath(), f.Size(), f.ReadMetadata())
		return errors.Wrap(uploadDataCaptureFile(ctx, s.client, f, s.partID), errMetadata)
	}

	if err := exponentialRetry(s.cancelCtx, fun, s.logger); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			s.logger.Error(errors.Wrap(closeErr, "error closing data capture file").Error())
		}

		// if we stopped due to a cancelled context,
		// return without deleting the file or moving it to the failed directory
		if errors.Is(err, context.Canceled) {
			return
		}

		s.logger.Error(err.Error())

		// otherwise we hit a terminal error, and we should move the file to the failed directory
		if err := moveFailedData(f.GetPath(), s.captureDir); err != nil {
			s.logger.Error(err)
		}
		return
	}

	if err := f.Delete(); err != nil {
		s.logger.Error(errors.Wrap(err, "error deleting data capture file").Error())
		return
	}
}

func (s *Syncer) syncArbitraryFile(f *os.File) {
	s.logger.Info("syncArbitraryFile START")
	defer s.logger.Info("syncArbitraryFile END")
	fun := func(ctx context.Context) error {
		errMetadata := fmt.Sprintf("error uploading file %s", f.Name())
		err := errors.Wrap(uploadArbitraryFile(ctx, s.client, f, s.partID, s.arbitraryFileTags, s.fileLastModifiedMillis, s.clock), errMetadata)

		if !isRetryableGRPCError(err) {
			if err := moveFailedData(f.Name(), path.Dir(f.Name())); err != nil {
				s.logger.Error(err)
			}
		}
		return err
	}

	if err := exponentialRetry(s.cancelCtx, fun, s.logger); err != nil {
		err := f.Close()
		if err != nil {
			s.logger.Error(errors.Wrap(err, "error closing data capture file").Error())
		}
		return
	}
	if err := os.Remove(f.Name()); err != nil {
		s.logger.Error(errors.Wrap(err, fmt.Sprintf("error deleting file %s", f.Name())).Error())
		return
	}
}

// MarkInProgress marks path as in progress in s.inProgress. It returns true if it changed the progress status,
// or false if the path was already in progress.
func (s *Syncer) MarkInProgress(path string) bool {
	s.progressLock.Lock()
	defer s.progressLock.Unlock()
	if s.inProgress[path] {
		// s.logger.Debugw("File already in progress, trying to mark it again", "file", path)
		return false
	}
	s.inProgress[path] = true
	return true
}

func (s *Syncer) InProgress(path string) bool {
	s.progressLock.Lock()
	defer s.progressLock.Unlock()
	return s.inProgress[path]
}

// UnmarkInProgress unmarks a path as in progress in s.inProgress.
func (s *Syncer) UnmarkInProgress(path string) {
	s.progressLock.Lock()
	defer s.progressLock.Unlock()
	delete(s.inProgress, path)
}

// exponentialRetry calls fn and retries with exponentially increasing waits from initialWait to a
// maximum of maxRetryInterval.
func exponentialRetry(
	ctx context.Context,
	fn func(ctx context.Context) error,
	logger logging.Logger,
) error {
	// Only create a ticker and enter the retry loop if we actually need to retry.
	err := fn(ctx)
	if err == nil {
		logger.Debug("succeeded")
		return nil
	}

	// Don't retry non-retryable errors.
	if !isRetryableGRPCError(err) {
		logger.Debugf("non retryable error: %s", err.Error())
		return err
	}
	logger.Debugf("retryable error: %s", err.Error())

	// First call failed, so begin exponentialRetry with a factor of RetryExponentialFactor
	nextWait := time.Millisecond * time.Duration(initialWaitTimeMillis)
	logger.Infof("nextWait: %s", nextWait)
	ticker := time.NewTicker(nextWait)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		logger.Debug("waiting")
		select {
		// If cancelled, return nil.
		case <-ctx.Done():
			return ctx.Err()
			// Otherwise, try again after nextWait.
		case <-ticker.C:
			logger.Debug("tick")
			if err := fn(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					logger.Error(err.Error())
				}
				// If error, retry with a new nextWait.
				offline := isOfflineGRPCError(err)
				nextWait = getNextWait(nextWait, offline)
				logger.Infof("next nextWait: %s, offline: %t", nextWait, offline)
				ticker.Reset(nextWait)
				continue
			}
			// If no error, return.
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
		return errors.Wrapf(err, fmt.Sprintf("error getting relative path between: %s and %s", parentDir, path))
	}
	// Create a new directory parentDir/corrupted/pathToFile
	newDir := filepath.Join(parentDir, FailedDir, filepath.Dir(relativePath))
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return errors.Wrapf(err, fmt.Sprintf("error making new directory: %s", newDir))
	}
	// Move the file from parentDir/pathToFile/file.ext to parentDir/corrupted/pathToFile/file.ext
	newPath := filepath.Join(newDir, filepath.Base(path))
	if err := os.Rename(path, newPath); err != nil {
		return errors.Wrapf(err, fmt.Sprintf("error moving: %s to %s", path, newPath))
	}
	return nil
}

func getNextWait(lastWait time.Duration, isOffline bool) time.Duration {
	if lastWait == time.Duration(0) {
		return time.Millisecond * time.Duration(initialWaitTimeMillis)
	}

	if isOffline {
		return time.Second * time.Duration(offlineWaitTimeSeconds)
	}

	nextWait := lastWait * time.Duration(retryExponentialFactor)
	if nextWait > maxRetryInterval {
		return maxRetryInterval
	}
	return nextWait
}
