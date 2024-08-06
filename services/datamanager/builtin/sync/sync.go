// Package sync implements datasync for the builtin datamanger
package sync

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/utils"
)

// DeletionTicker temporarily public for tests.
var errGrpcClientConnUnrecoverable = errors.New("can't turn s.cloudConn into a grpc.ClientConn")

const (
	// FilesystemPollInterval temporarily public for tests.
	FilesystemPollInterval = 30 * time.Second
	// FailedDir is a subdirectory of the capture directory that holds any files that could not be synced.
	FailedDir = "failed"
	// InitialWaitTimeMillis defines the time to wait on the first retried upload attempt.
	initialWaitTimeMillis = 200
	// RetryExponentialFactor defines the factor by which the retry wait time increases.
	retryExponentialFactor = 2
	// OfflineWaitTimeSeconds defines the amount of time to wait to retry if the machine is offline.
	offlineWaitTimeSeconds = 60
	maxRetryInterval       = time.Hour
	grpcConnectionTimeout  = 10 * time.Second
)

// Sync manages uploading metrics from files to the cloud & deleting the upload files.
// There must be only one Sync per DataManager. The lifecycle of a Capture is:
//
// - NewSync
// - Reconfigure (any number of times)
// - Close (once).
type Sync struct {
	logger          logging.Logger
	workersWg       sync.WaitGroup
	flushCollectors func()
	fileTracker     *fileTracker
	filesToSync     chan string

	configMu sync.Mutex
	config   Config

	configCtx        context.Context
	configCancelFunc func()

	cloudSvc cloudSvc

	cloudConn cloudConn

	scheduler           utils.StoppableWorkers
	cloudConnManager    utils.StoppableWorkers
	fileDeletingWorkers utils.StoppableWorkers
}

// New creates a new Manager.
func New(
	clientConstructor func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient,
	flushCollectors func(),
	logger logging.Logger,
) *Sync {
	s := Sync{
		logger:              logger,
		fileTracker:         newFileTracker(),
		filesToSync:         make(chan string),
		flushCollectors:     flushCollectors,
		scheduler:           utils.NewStoppableWorkers(),
		cloudSvc:            cloudSvc{ready: make(chan struct{})},
		cloudConn:           cloudConn{ready: make(chan struct{})},
		fileDeletingWorkers: utils.NewStoppableWorkers(),
	}
	s.cloudConnManager = utils.NewStoppableWorkers(func(ctx context.Context) {
		s.runCloudConnManager(ctx, clientConstructor)
	})
	return &s
}

// Reconfigure reconfigures Sync.
// It is only called by the builtin data manager
// https://github.com/dgottlieb/rdk/blob/72f5b567db2cb2ca08b9752b8710d1e4e784077c/services/datamanager/datasync/manager.go
// https://github.com/dgottlieb/rdk/blob/72f5b567db2cb2ca08b9752b8710d1e4e784077c/services/datamanager/builtin/builtin.go#L144
// Reconfigure:
// 1. stops all workers which use the config
// 2. sets the cloud.ConnectionService if it hans't been set yet (only needs to be set once)
// and notifies the cloud connection manager so it can make a cloud connection
// 3. starts up the appropriate workers which use the new config.
func (s *Sync) Reconfigure(_ context.Context, config Config, cloudConnSvc cloud.ConnectionService) {
	if s.config.equal(config) && s.cloudSvc.cloudConnSvc != nil {
		// if config has not changed and cloudConnSvc is not nil then reconfigure doesn't need
		// to execute, don't stop workers
		return
	}
	s.configCancelFunc()
	s.fileDeletingWorkers.Stop()
	s.scheduler.Stop()
	s.workersWg.Wait()

	s.configMu.Lock()
	s.config = config
	s.configMu.Unlock()
	// set up nwe config
	s.configCtx, s.configCancelFunc = context.WithCancel(context.Background())
	// set cloud connection service if we don't have one yet
	if s.cloudSvc.cloudConnSvc == nil {
		s.cloudSvc.cloudConnSvc = cloudConnSvc
		close(s.cloudSvc.ready)
	}

	// start workers
	s.startWorkers(config)
	if config.schedulerEnabled() {
		s.scheduler = utils.NewStoppableWorkers(func(ctx context.Context) {
			s.runScheduler(ctx, config)
		})
	}

	// if datacapture is enabled, kick off a go routine to handle disk space filling due to
	// cached datacapture files
	shouldDeleteExcessFiles := !config.CaptureDisabled
	if shouldDeleteExcessFiles {
		s.fileDeletingWorkers = utils.NewStoppableWorkers(func(ctx context.Context) {
			deleteExcessFiles(
				ctx,
				s.fileTracker,
				config.CaptureDir,
				config.DeleteEveryNthWhenDiskFull,
				s.logger,
			)
		})
	}
}

// Close releases all resources managed by data_manager.
func (s *Sync) Close() {
	s.configCancelFunc()
	s.fileDeletingWorkers.Stop()
	s.scheduler.Stop()
	s.workersWg.Wait()
	s.cloudConnManager.Stop()
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
// If automated sync is also enabled, calling Sync will upload the files,
// regardless of whether or not is the scheduled time.
func (s *Sync) Sync(ctx context.Context, _ map[string]interface{}) error {
	// NOTE: The current implementation doesn't check if the robot is currently connected
	// to app, just that it has a connection that MIGHT Be connected
	select {
	case <-s.cloudConn.ready:
	default:
		return errors.New("not connected to the cloud")
	}
	s.configMu.Lock()
	config := s.config
	s.configMu.Unlock()
	return s.sendFilesToSync(ctx, config)
}

type cloudConn struct {
	// closed by cloud conn manager
	ready    chan struct{}
	partID   string
	client   v1.DataSyncServiceClient
	conn     rpc.ClientConn
	grpcConn *rpc.GrpcOverHTTPClientConn
}
type cloudSvc struct {
	// closed by reconfigure
	ready        chan struct{}
	cloudConnSvc cloud.ConnectionService
}

// Ensures that a cloud connection is established.
// Handles closing & recreating a cloud connection if the cloud service ever changes
// Lives for the lifetime of the Sync.
func (s *Sync) runCloudConnManager(
	ctx context.Context,
	clientConstructor func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient,
) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		// wait until either the syncer is closed or we have a cloud service
		select {
		case <-ctx.Done():
			return
		case <-s.cloudSvc.ready:
		}

		// once we have a cloud service we determine if we can get a cloud connection
		partID, conn, grpcConn, err := newCloudConn(ctx, s.cloudSvc.cloudConnSvc)
		if errors.Is(err, errGrpcClientConnUnrecoverable) {
			// if it is impossible we log an irricoverable error & give up
			s.logger.Error("datamanager can't sync due to unrecoverable error: " + err.Error())
			return
		}

		if errors.Is(err, cloud.ErrNotCloudManaged) {
			// if we are running in a non cloud managed robot, give up
			// TODO: Communicate to the syncer that we are operating in a non cloud managed robot
			// What is the desired behavior of the syncer in this case
			s.logger.Error("datamanager can't sync as the robot is not cloud managed: " + err.Error())
			return
		}

		if err != nil {
			continue
		}

		// we have a working cloudConn,
		// set the values & connunicate that it is ready
		s.cloudConn.partID = partID
		s.cloudConn.conn = conn
		s.cloudConn.grpcConn = grpcConn
		s.cloudConn.client = clientConstructor(conn)
		close(s.cloudConn.ready)
		// now that we have a connection we wait until the connecivity manager is cancelled
		break
	}
	<-ctx.Done()
	// at which point we close the connetion
	goutils.UncheckedError(s.cloudConn.conn.Close())
}

func newCloudConn(
	ctx context.Context,
	cloudConnSvc cloud.ConnectionService,
) (string, rpc.ClientConn, *rpc.GrpcOverHTTPClientConn, error) {
	grpcCtx, grpcCancel := context.WithTimeout(ctx, grpcConnectionTimeout)
	defer grpcCancel()
	partID, conn, err := cloudConnSvc.AcquireConnection(grpcCtx)
	if err != nil {
		return "", nil, nil, err
	}

	grpcConn, ok := conn.(*rpc.GrpcOverHTTPClientConn)
	if !ok {
		goutils.UncheckedError(conn.Close())
		return "", nil, nil, errGrpcClientConnUnrecoverable
	}

	return partID, conn, grpcConn, nil
}

// Assumed to be called after reconfigure is called.
func (s *Sync) startWorkers(config Config) {
	numThreads := config.MaximumNumSyncThreads
	s.logger.Infof("Making new syncer with %d max threads", numThreads)
	for i := 0; i < numThreads; i++ {
		s.workersWg.Add(1)
		goutils.ManagedGo(func() { s.runWorker(config) }, s.workersWg.Done)
	}
}

func (s *Sync) runWorker(config Config) {
	for {
		if s.configCtx.Err() != nil {
			return
		}
		select {
		case <-s.configCtx.Done():
			return
		case path := <-s.filesToSync:
			s.syncFile(config, path)
		}
	}
}

func (s *Sync) sendToSync(ctx context.Context, path string) {
	select {
	case <-ctx.Done():
		return
	case <-s.configCtx.Done():
		return
	case s.filesToSync <- path:
		return
	}
}

func (s *Sync) runScheduler(ctx context.Context, config Config) {
	// time.Duration loses precision at low floating point values, so turn intervalMins to milliseconds.
	intervalMillis := 60000.0 * config.SyncIntervalMins
	// The ticker must be created before uploadData returns to prevent race conditions between clock.Ticker and
	// clock.Add in sync_test.go.
	tkr := time.NewTimer(time.Millisecond * time.Duration(intervalMillis))
	defer tkr.Stop()
	s.logger.Infof("runScheduler START %p", tkr)
	defer s.logger.Info("runScheduler END")

	for {
		if err := ctx.Err(); err != nil {
			return
		}

		// wait for the cloud connection to be ready
		// or the scheduler to be cancelled
		select {
		case <-ctx.Done():
			return
		case <-s.cloudConn.ready:
		}

		select {
		case <-ctx.Done():
			return
		case <-tkr.C:
			// If selective sync is disabled, sync. If it is enabled, check the condition below.
			shouldSync := config.SelectiveSyncSensor == nil
			// If selective sync is enabled and the sensor has been properly initialized,
			// try to get the reading from the selective sensor that indicates whether to sync
			if config.SelectiveSyncSensor != nil {
				shouldSync = readyToSync(ctx, config.SelectiveSyncSensor, s.logger)
			}

			if online := s.cloudConn.grpcConn.GetState() == connectivity.Ready; online && shouldSync {
				goutils.UncheckedError(s.sendFilesToSync(ctx, config))
			}
		}
	}
}

// returns early with an error if either ctx is cancelled or if the reconfigure is called
// while sendFilesToSync.
func (s *Sync) sendFilesToSync(ctx context.Context, config Config) error {
	// Retrieve all files in capture dir and send them to the syncer
	f := func(path string, info os.FileInfo, err error) error {
		if ctx.Err() != nil {
			// if the context is cancelled, bail out
			s.logger.Info(path + " err: " + ctx.Err().Error())
			return filepath.SkipAll
		}

		if s.configCtx.Err() != nil {
			s.logger.Info(path + " err: " + ctx.Err().Error())
			return filepath.SkipAll
		}

		if err != nil {
			s.logger.Info(path + " err: " + err.Error())
			//nolint:nilerr
			// NOTE: Nick: This will ignore errors
			// read the docs for filepath.WalkFunc & determine if this is really
			// the strategy we want
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
		// TODO: Remove this when we remove s.clock
		timeSinceMod := time.Since(info.ModTime())
		// When using a mock clock in tests, this can be negative since the file system will still use the system clock.
		// Take max(timeSinceMod, 0) to account for this.
		if timeSinceMod < 0 {
			timeSinceMod = 0
		}
		isCompletedCaptureFile := filepath.Ext(path) == datacapture.FileExt
		isNonCaptureFileThatIsNotBeingWrittenTo := filepath.Ext(path) != datacapture.InProgressFileExt &&
			filepath.Ext(path) != datacapture.FileExt &&
			timeSinceMod >= time.Duration(config.FileLastModifiedMillis)*time.Millisecond
		if isCompletedCaptureFile || isNonCaptureFileThatIsNotBeingWrittenTo && !s.fileTracker.inProgress(path) {
			s.sendToSync(ctx, path)
		}
		return nil
	}

	s.flushCollectors()
	var errs []error
	for _, dir := range config.syncPaths() {
		errs = append(errs, filepath.Walk(dir, f))
	}
	errs = append(errs, ctx.Err(), s.configCtx.Err())
	return multierr.Combine(errs...)
}

func (s *Sync) syncFile(config Config, path string) {
	// s.logger.Infof("syncFile: %s", path)
	// If the file is already being synced, do not kick off a new goroutine.
	// The goroutine will again check and return early if sync is already in progress.
	if !s.fileTracker.markInProgress(path) {
		// s.logger.Warnf("syncFile already in progress %s", path)
		return
	}
	defer s.fileTracker.unmarkInProgress(path)
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
			if err := moveFailedData(f.Name(), config.CaptureDir); err != nil {
				s.logger.Error(err)
			}
			return
		}
		syncDataCaptureFile(
			s.configCtx,
			captureFile,
			s.cloudConn,
			config.CaptureDir,
			s.logger,
		)
	} else {
		s.logger.Infof("ArbitraryFile: %s", path)
		// todo: config that Tags is right here
		syncArbitraryFile(
			s.configCtx,
			f,
			s.cloudConn,
			config.Tags,
			config.FileLastModifiedMillis,
			s.logger,
		)
	}
}

func syncDataCaptureFile(
	configCtx context.Context,
	f *datacapture.File,
	conn cloudConn,
	captureDir string,
	logger logging.Logger,
) {
	fun := func(ctx context.Context) error {
		errMetadata := fmt.Sprintf("error uploading data capture file %s, size: %d, md: %s", f.GetPath(), f.Size(), f.ReadMetadata())
		return errors.Wrap(uploadDataCaptureFile(ctx, f, conn), errMetadata)
	}

	if err := exponentialRetry(configCtx, fun, logger); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			logger.Error(errors.Wrap(closeErr, "error closing data capture file").Error())
		}

		// if we stopped due to a cancelled context,
		// return without deleting the file or moving it to the failed directory
		if errors.Is(err, context.Canceled) {
			return
		}

		logger.Error(err.Error())

		// otherwise we hit a terminal error, and we should move the file to the failed directory
		if err := moveFailedData(f.GetPath(), captureDir); err != nil {
			logger.Error(err)
		}
		return
	}

	if err := f.Delete(); err != nil {
		logger.Error(errors.Wrap(err, "error deleting data capture file").Error())
		return
	}
}

func syncArbitraryFile(
	configCtx context.Context,
	f *os.File,
	conn cloudConn,
	arbitraryFileTags []string,
	fileLastModifiedMillis int,
	logger logging.Logger,
) {
	logger.Info("syncArbitraryFile START")
	defer logger.Info("syncArbitraryFile END")
	fun := func(ctx context.Context) error {
		errMetadata := fmt.Sprintf("error uploading file %s", f.Name())
		err := errors.Wrap(uploadArbitraryFile(ctx, f, conn, arbitraryFileTags, fileLastModifiedMillis), errMetadata)

		if !isRetryableGRPCError(err) {
			if err := moveFailedData(f.Name(), path.Dir(f.Name())); err != nil {
				logger.Error(err)
			}
		}
		return err
	}

	if err := exponentialRetry(configCtx, fun, logger); err != nil {
		err := f.Close()
		if err != nil {
			logger.Error(errors.Wrap(err, "error closing data capture file").Error())
		}
		return
	}
	if err := os.Remove(f.Name()); err != nil {
		logger.Error(errors.Wrap(err, fmt.Sprintf("error deleting file %s", f.Name())).Error())
		return
	}
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

// readyToSync is a method for getting the bool reading from the selective sync sensor
// for determining whether the key is present and what its value is.
func readyToSync(ctx context.Context, s sensor.Sensor, logger logging.Logger) (readyToSync bool) {
	readyToSync = false
	readings, err := s.Readings(ctx, nil)
	if err != nil {
		logger.CErrorw(ctx, "error getting readings from selective syncer", "error", err.Error())
		return
	}
	readyToSyncVal, ok := readings[datamanager.ShouldSyncKey]
	if !ok {
		logger.CErrorf(ctx, "value for should sync key %s not present in readings", datamanager.ShouldSyncKey)
		return
	}
	readyToSyncBool, err := utils.AssertType[bool](readyToSyncVal)
	if err != nil {
		logger.CErrorw(ctx, "error converting should sync key to bool", "key", datamanager.ShouldSyncKey, "error", err.Error())
		return
	}
	readyToSync = readyToSyncBool
	return
}

func deleteExcessFiles(
	ctx context.Context,
	fileTracker *fileTracker,
	captureDir string,
	deleteEveryNth int,
	logger logging.Logger,
) {
	if runtime.GOOS == "android" {
		logger.Debug("file deletion if disk is full is not currently supported on Android")
		return
	}
	t := time.NewTicker(FilesystemPollInterval)
	defer t.Stop()
	for {
		if err := ctx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Errorw("data manager context closed unexpectedly in filesystem polling", "error", err)
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			logger.Debug("checking disk usage")
			shouldDelete, err := shouldDeleteBasedOnDiskUsage(ctx, captureDir, logger)
			if err != nil {
				logger.Warnw("error checking file system stats", "error", err)
			}
			if shouldDelete {
				start := time.Now()
				deletedFileCount, err := deleteFiles(ctx, fileTracker, deleteEveryNth, captureDir, logger)
				duration := time.Since(start)
				if err != nil {
					logger.Errorw("error deleting cached datacapture files", "error", err, "execution time", duration.Seconds())
				} else {
					logger.Infof("%v files have been deleted to avoid the disk filling up, execution time: %f", deletedFileCount, duration.Seconds())
				}
			}
		}
	}
}
