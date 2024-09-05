// Package sync implements datasync for the builtin datamanger
package sync

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/utils"
)

// CheckDeleteExcessFilesInterval temporarily public for tests.
var CheckDeleteExcessFilesInterval = 30 * time.Second

const (
	// FailedDir is a subdirectory of the capture directory that holds any files that could not be synced.
	FailedDir = "failed"
	// grpcConnectionTimeout defines the timeout for getting a connection with app.viam.com.
	grpcConnectionTimeout = 10 * time.Second
	// durationBetweenAcquireConnection defines how long to wait after a call to cloud.AcquireConnection fails
	// with a transient error.
	durationBetweenAcquireConnection = time.Second
)

// Sync manages uploading files (both written by data capture and by 3rd party applications)
// to the cloud & deleting the upload files.
// It also manages deleting files if capture is enabled and the disk is about to fill up.
// There must be only one Sync per DataManager. The lifecycle of a Sync is:
//
// - New
// - Reconfigure (any number of times)
// - Close (once).
type Sync struct {
	// ScheduledTicker only exists for tests
	ScheduledTicker         *clock.Ticker
	connToConnectivityState func(conn rpc.ClientConn) ConnectivityState
	logger                  logging.Logger
	workersWg               sync.WaitGroup
	flushCollectors         func()
	fileTracker             *fileTracker
	filesToSync             chan string
	clientConstructor       func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient
	clock                   clock.Clock

	configMu sync.Mutex
	config   Config

	configCtx        context.Context
	configCancelFunc func()

	cloudConn cloudConn

	Scheduler        *goutils.StoppableWorkers
	cloudConnManager *goutils.StoppableWorkers
	// FileDeletingWorkers is only public for tests
	FileDeletingWorkers *goutils.StoppableWorkers
	// MaxSyncThreads only exists for tests
	MaxSyncThreads int
}

// New creates a new Sync.
func New(
	clientConstructor func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient,
	connToConnectivityState func(conn rpc.ClientConn) ConnectivityState,
	flushCollectors func(),
	clock clock.Clock,
	logger logging.Logger,
) *Sync {
	configCtx, configCancelFunc := context.WithCancel(context.Background())
	s := Sync{
		connToConnectivityState: connToConnectivityState,
		clock:                   clock,
		configCtx:               configCtx,
		configCancelFunc:        configCancelFunc,
		clientConstructor:       clientConstructor,
		logger:                  logger,
		fileTracker:             newFileTracker(),
		filesToSync:             make(chan string),
		flushCollectors:         flushCollectors,
		Scheduler:               goutils.NewBackgroundStoppableWorkers(),
		cloudConn:               cloudConn{ready: make(chan struct{})},
		FileDeletingWorkers:     goutils.NewBackgroundStoppableWorkers(),
	}
	return &s
}

// Reconfigure reconfigures Sync and is only called by the builtin data manager
// it assumes that it is only called by one goroutine at a time.
// Reconfigure:
// 1. stops all workers which use the config
// 2. sets the cloud.ConnectionService if it hans't been set yet (only needs to be set once)
// and starts the cloud connection manager if it hasn't been started yet so it can make a cloud connection
// 3. starts up the appropriate workers which use the new config.
func (s *Sync) Reconfigure(_ context.Context, config Config, cloudConnSvc cloud.ConnectionService) {
	s.logger.Debug("Reconfigure START")
	defer s.logger.Debug("Reconfigure END")
	if s.cloudConnManager == nil {
		s.cloudConnManager = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
			s.runCloudConnManager(ctx, cloudConnSvc)
		})
	}
	if s.config.Equal(config) {
		// if config has not changed then reconfigure doesn't need
		// to execute, don't stop workers
		return
	}

	// config changed... stop workers
	s.configCancelFunc()
	s.FileDeletingWorkers.Stop()
	s.Scheduler.Stop()
	s.ScheduledTicker = nil
	// wait for workers to stop
	s.workersWg.Wait()

	oldSyncPaths := strings.Join(s.config.syncPaths(), " ")
	newSyncPaths := strings.Join(config.syncPaths(), " ")
	if newSyncPaths != oldSyncPaths {
		s.logger.Infof("sync_paths: %s", newSyncPaths)
	}

	// update config
	s.configMu.Lock()
	s.config = config
	s.configMu.Unlock()
	// reset config context
	s.configCtx, s.configCancelFunc = context.WithCancel(context.Background())

	// start workers
	s.startWorkers(config)
	if config.schedulerEnabled() {
		// time.Duration loses precision at low floating point values, so turn intervalMins to milliseconds.
		intervalMillis := 60000.0 * config.SyncIntervalMins
		// The ticker must be created before uploadData returns to prevent race conditions between clock.Ticker and
		// clock.Add in sync_test.go.
		interval := time.Millisecond * time.Duration(intervalMillis)
		tkr := s.clock.Ticker(interval)
		s.ScheduledTicker = tkr
		s.Scheduler = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
			s.runScheduler(ctx, tkr, config)
		})
	} else {
		s.logger.Info("Sync Disabled")
	}

	// if datacapture is enabled, kick off a go routine to handle disk space filling due to
	// cached datacapture files
	shouldDeleteExcessFiles := !config.CaptureDisabled
	if shouldDeleteExcessFiles {
		s.FileDeletingWorkers = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
			deleteExcessFilesOnSchedule(
				ctx,
				s.fileTracker,
				config.CaptureDir,
				config.DeleteEveryNthWhenDiskFull,
				s.clock,
				s.logger,
			)
		})
	}
}

// Close releases all resources managed by data_manager.
func (s *Sync) Close() {
	s.configCancelFunc()
	s.FileDeletingWorkers.Stop()
	s.Scheduler.Stop()
	s.workersWg.Wait()
	if s.cloudConnManager != nil {
		s.cloudConnManager.Stop()
	}
}

// CloudConnReady is public for builtin tests.
func (s *Sync) CloudConnReady() chan struct{} {
	return s.cloudConn.ready
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
// If automated sync is also enabled, calling Sync will upload the files,
// regardless of whether or not is the scheduled time.
func (s *Sync) Sync(ctx context.Context, _ map[string]interface{}) error {
	select {
	case <-s.cloudConn.ready:
	default:
		return errors.New("not connected to the cloud")
	}
	s.configMu.Lock()
	config := s.config
	s.configMu.Unlock()
	return s.walkDirsAndSendFilesToSync(ctx, config)
}

type cloudConn struct {
	// closed by cloud conn manager
	ready                        chan struct{}
	partID                       string
	client                       v1.DataSyncServiceClient
	conn                         rpc.ClientConn
	connectivityStateEnabledConn ConnectivityState
}

// BEGIN connection management

// Ensures that a cloud connection is established.
// Handles creating a cloud connection from the cloud service
// and notifying Sync when the cloud connection is ready.
// Also handles closing the cloud connection once Close is called
// Lives for the lifetime of the Sync insance.
func (s *Sync) runCloudConnManager(
	ctx context.Context,
	cloudConnSvc cloud.ConnectionService,
) {
	for {
		// if context is canelled, sync is shutting down,
		// terminate
		if err := ctx.Err(); err != nil {
			return
		}

		// once we have a cloud service we determine if we can get a cloud connection
		s.logger.Debug("attempting to acquire cloud connection")
		partID, conn, err := newCloudConn(ctx, cloudConnSvc)

		if errors.Is(err, cloud.ErrNotCloudManaged) {
			// if we are running in a non cloud managed robot, give up
			// this will block syncing to the cloud as s.cloudConn.ready will never
			// close, which will mean that both manual sync & scheduled sync
			// won't run
			s.logger.Error("datamanager can't sync as the robot is not cloud managed: " + err.Error())
			return
		}

		// this is a recoverable error, most likely we are offline,
		// continue retrying until newCloudConn succeeds or sync
		// shuts down
		if err != nil {
			s.logger.Debugf("hit transient error trying to get cloud connection, "+
				"will retry in %s err: %s", durationBetweenAcquireConnection, err.Error())
			if goutils.SelectContextOrWait(ctx, durationBetweenAcquireConnection) {
				continue
			}
			// exit loop if context is cancelled
			return
		}

		// we have a working cloudConn,
		// set the values & connunicate that it is ready
		s.cloudConn.partID = partID
		s.cloudConn.conn = conn
		s.cloudConn.connectivityStateEnabledConn = s.connToConnectivityState(conn)
		s.cloudConn.client = s.clientConstructor(conn)
		close(s.cloudConn.ready)
		// now that we have a connection ...
		break
	}
	// we wait until the connecivity manager is cancelled
	<-ctx.Done()
	// at which point we close the connetion
	goutils.UncheckedError(s.cloudConn.conn.Close())
}

func newCloudConn(
	ctx context.Context,
	cloudConnSvc cloud.ConnectionService,
) (string, rpc.ClientConn, error) {
	grpcCtx, grpcCancel := context.WithTimeout(ctx, grpcConnectionTimeout)
	defer grpcCancel()
	partID, conn, err := cloudConnSvc.AcquireConnection(grpcCtx)
	if err != nil {
		return "", nil, err
	}

	return partID, conn, nil
}

// END connection management

// BEGIN sync workers
// Assumed to be called after reconfigure is called.
func (s *Sync) startWorkers(config Config) {
	numThreads := config.MaximumNumSyncThreads
	s.MaxSyncThreads = numThreads
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

func (s *Sync) syncFile(config Config, filePath string) {
	// don't sync in progress files
	if filepath.Ext(filePath) == data.InProgressCaptureFileExt {
		s.logger.Warn("ignoreing request to sync in progress capture file: %s", filePath)
		return
	}

	// If the file is already being synced, do not kick off a new goroutine.
	// The goroutine will again check and return early if sync is already in progress.
	if !s.fileTracker.markInProgress(filePath) {
		s.logger.Warn("ignoreing request to sync file which sync is already working on %s", filePath)
		return
	}
	defer s.fileTracker.unmarkInProgress(filePath)
	//nolint:gosec
	f, err := os.Open(filePath)
	if err != nil {
		// Don't log if the file does not exist, because that means it was successfully synced and deleted
		// in between paths being built and this executing.
		if !errors.Is(err, os.ErrNotExist) {
			s.logger.Errorw("error opening file", "error", err)
		}
		return
	}

	if data.IsDataCaptureFile(f) {
		s.syncDataCaptureFile(f, config.CaptureDir, s.logger)
	} else {
		s.syncArbitraryFile(f, config.Tags, config.FileLastModifiedMillis, s.logger)
	}
}

const (
	_ = 1 << (10 * iota)
	kib
	mib
	gib
	tib
)

func formatBytesI64(b int64) string {
	switch {
	case b > tib:
		return fmt.Sprintf("%.2f TB", float64(b)/tib)
	case b > gib:
		return fmt.Sprintf("%.2f GB", float64(b)/gib)
	case b > mib:
		return fmt.Sprintf("%.2f MB", float64(b)/mib)
	case b > kib:
		return fmt.Sprintf("%.2f KB", float64(b)/kib)
	default:
		return fmt.Sprintf("%d Bytes", b)
	}
}

func (s *Sync) syncDataCaptureFile(f *os.File, captureDir string, logger logging.Logger) {
	captureFile, err := data.ReadCaptureFile(f)
	// if you can't read the capture file's metadata field, close & move it to the failed directory
	if err != nil {
		cause := errors.Wrap(err, "ReadCaptureFile failed")

		if err := f.Close(); err != nil {
			logger.Error(errors.Wrapf(err, "failed to close file %s", f.Name()).Error())
		}

		if err := moveFailedData(f.Name(), captureDir, cause, logger); err != nil {
			s.logger.Error(err)
		}
		return
	}

	// setup a retry struct that will try to upload the capture file
	retry := newExponentialRetry(s.configCtx, s.clock, s.logger, f.Name(), func(ctx context.Context) error {
		msg := "error uploading data capture file %s, size: %s, md: %s"
		errMetadata := fmt.Sprintf(msg, captureFile.GetPath(), formatBytesI64(captureFile.Size()), captureFile.ReadMetadata())
		return errors.Wrap(uploadDataCaptureFile(ctx, captureFile, s.cloudConn, logger), errMetadata)
	})

	if err := retry.run(); err != nil {
		// if unable to upload the capture file
		if closeErr := captureFile.Close(); closeErr != nil {
			logger.Error(errors.Wrap(closeErr, "error closing data capture file").Error())
		}

		// if we stopped due to a cancelled context,
		// return without deleting the file or moving it to the failed directory
		if errors.Is(err, context.Canceled) {
			return
		}

		// otherwise we hit a terminal error, and we should move the file to the failed directory
		if err := moveFailedData(captureFile.GetPath(), captureDir, err, logger); err != nil {
			logger.Error(err)
		}
		return
	}

	// file was successfully uploaded, delete it and log an error if unable to delete
	if err := captureFile.Delete(); err != nil {
		logger.Error(errors.Wrap(err, "error deleting data capture file").Error())
	}
}

func (s *Sync) syncArbitraryFile(f *os.File, tags []string, fileLastModifiedMillis int, logger logging.Logger) {
	retry := newExponentialRetry(s.configCtx, s.clock, s.logger, f.Name(), func(ctx context.Context) error {
		errMetadata := fmt.Sprintf("error uploading arbitrary file %s", f.Name())
		return errors.Wrap(uploadArbitraryFile(ctx, f, s.cloudConn, tags, fileLastModifiedMillis, s.clock, logger), errMetadata)
	})

	if err := retry.run(); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			logger.Error(errors.Wrap(closeErr, "error closing data capture file").Error())
		}

		// if we stopped due to a cancelled context,
		// return without deleting the file or moving it to the failed directory
		if errors.Is(err, context.Canceled) {
			return
		}

		// otherwise we hit a terminal error, and we should move the file to the failed directory
		if err := moveFailedData(f.Name(), path.Dir(f.Name()), err, logger); err != nil {
			logger.Error(err.Error())
		}
		return
	}

	if err := f.Close(); err != nil {
		logger.Error(errors.Wrap(err, "error closing arbitrary file").Error())
	}

	if err := os.Remove(f.Name()); err != nil {
		logger.Error(errors.Wrap(err, fmt.Sprintf("error deleting file %s", f.Name())).Error())
	}
}

// moveFailedData takes any data that could not be synced in the parentDir and
// moves it to a new subdirectory "failed" that will not be synced.
func moveFailedData(path, parentDir string, cause error, logger logging.Logger) error {
	// Remove the parentDir part of the path to the corrupted data
	relativePath, err := filepath.Rel(parentDir, path)
	if err != nil {
		return errors.Wrapf(err, "failed to move file to failed directory: error getting relative path between: %s and %s", parentDir, path)
	}
	// Create a new directory parentDir/corrupted/pathToFile
	newDir := filepath.Join(parentDir, FailedDir, filepath.Dir(relativePath))
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return errors.Wrapf(err, "failed to move file to failed directory: error making new failed directory: %s", newDir)
	}
	// Move the file from parentDir/pathToFile/file.ext to parentDir/corrupted/pathToFile/file.ext
	newPath := filepath.Join(newDir, filepath.Base(path))
	logger.Warnf("moving file that data manager failed to sync due to err: %v, from %s to %s", cause, path, newPath)
	if err := os.Rename(path, newPath); err != nil {
		return errors.Wrapf(err, "failed to move file to failed directory: error moving: %s to %s", path, newPath)
	}
	return nil
}

// END sync workers

// BEGIN sync scheudler.
func (s *Sync) runScheduler(ctx context.Context, tkr *clock.Ticker, config Config) {
	defer tkr.Stop()
	var readyLogged bool

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
			if !readyLogged {
				s.logger.Info("cloud connection ready")
				readyLogged = true
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-tkr.C:
			shouldSync := readyToSyncDirectories(ctx, config, s.logger)
			state := s.cloudConn.connectivityStateEnabledConn.GetState()
			online := state == connectivity.Ready
			if !online {
				s.logger.Infof("data manager: NOT syncing data to the cloud as it's cloud connection is in state: %s"+
					"; waiting for it to be in state: %s", state, connectivity.Ready)
				continue
			}

			if !shouldSync {
				s.logger.Info("data manager: NOT syncing data to the cloud as it's selective sync sensor is not ready to sync")
				continue
			}

			if err := s.walkDirsAndSendFilesToSync(ctx, config); err != nil && !errors.Is(err, context.Canceled) {
				goutils.UncheckedError(err)
			}
		}
	}
}

// returns early with an error if either ctx is cancelled or if the reconfigure is called
// while walkDirsAndSendFilesToSync.
func (s *Sync) walkDirsAndSendFilesToSync(ctx context.Context, config Config) error {
	s.flushCollectors()
	var errs []error
	for _, dir := range config.syncPaths() {
		// Retrieve all files in capture dir and send them to the syncer
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err := ctx.Err(); err != nil {
				// if the context is cancelled, bail out
				return filepath.SkipAll
			}

			if err := s.configCtx.Err(); err != nil {
				return filepath.SkipAll
			}

			if err != nil {
				s.logger.Debugf("walkDirsAndSendFilesToSync ignoring error walking path: %s, err: %v", path, err)
				return nil
			}

			// Do not sync the files in the corrupted data directory.
			if info.IsDir() && info.Name() == FailedDir {
				return filepath.SkipDir
			}

			if info.IsDir() {
				return nil
			}

			// if the file is empty, no need to sync
			if info.Size() == 0 {
				return nil
			}
			// If a non data capture owned file was modified within the past lastModifiedMillis, do not sync it (data
			// may still be being written).
			// When using a mock clock in tests, s.clock.Since(info.ModTime()) can be negative since the file system will still use the system clock.
			// Take max(timeSinceMod, 0) to account for this.
			timeSinceMod := max(s.clock.Since(info.ModTime()), 0)
			if readyToSyncFile(timeSinceMod, path, config.FileLastModifiedMillis, s.fileTracker) {
				s.sendToSync(ctx, path)
			}
			return nil
		})
		errs = append(errs, err)
	}
	errs = append(errs, ctx.Err(), s.configCtx.Err())
	return multierr.Combine(errs...)
}

func readyToSyncFile(timeSinceMod time.Duration, path string, fileLastModifiedMillis int, fileTracker *fileTracker) bool {
	// if file is in progress, it is not ready to sync as some other goroutine is acting on it
	if fileTracker.inProgress(path) {
		return false
	}

	return isCompletedCaptureFile(path) ||
		isNonCaptureFileThatIsNotBeingWrittenTo(timeSinceMod, path, fileLastModifiedMillis)
}

func isCompletedCaptureFile(path string) bool {
	return filepath.Ext(path) == data.CompletedCaptureFileExt
}

func isNonCaptureFileThatIsNotBeingWrittenTo(timeSinceMod time.Duration, path string, fileLastModifiedMillis int) bool {
	return filepath.Ext(path) != data.InProgressCaptureFileExt &&
		filepath.Ext(path) != data.CompletedCaptureFileExt &&
		timeSinceMod >= time.Duration(fileLastModifiedMillis)*time.Millisecond
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

// readyToSyncDirectories is a method for getting the bool reading from the selective sync sensor
// for determining whether the key is present and what its value is.
func readyToSyncDirectories(ctx context.Context, config Config, logger logging.Logger) bool {
	// If selective sync is disabled, sync. If it is enabled, check the condition below.
	if !config.SelectiveSyncSensorEnabled {
		return true
	}

	// if there is no sync sensor, then you are ready to sync
	if config.SelectiveSyncSensor == nil {
		return true
	}

	// If selective sync is enabled and the sensor has been properly initialized,
	// try to get the reading from the selective sensor that indicates whether to sync
	readings, err := config.SelectiveSyncSensor.Readings(ctx, nil)
	if err != nil {
		logger.CErrorw(ctx, "error getting readings from selective syncer", "error", err.Error())
		return false
	}
	readyToSyncVal, ok := readings[datamanager.ShouldSyncKey]
	if !ok {
		logger.CErrorf(ctx, "value for should sync key %s not present in readings", datamanager.ShouldSyncKey)
		return false
	}
	readyToSyncBool, err := utils.AssertType[bool](readyToSyncVal)
	if err != nil {
		logger.CErrorw(ctx, "error converting should sync key to bool", "key", datamanager.ShouldSyncKey, "error", err.Error())
		return false
	}
	return readyToSyncBool
}

// END sync scheudler
