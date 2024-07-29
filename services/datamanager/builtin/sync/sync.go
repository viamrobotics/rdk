// Package sync implements datasync for the builtin datamanger
package sync

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	uatomic "go.uber.org/atomic"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/utils"
)

var (
	// DefaultDeleteEveryNth temporarily public for tests.
	DefaultDeleteEveryNth = 5
	// DeletionTicker temporarily public for tests.
	DeletionTicker = clk.New()
	// FilesystemPollInterval temporarily public for tests.
	FilesystemPollInterval = 30 * time.Second
)

type selectiveSyncer interface {
	sensor.Sensor
}

// Sync manages uploading metrics from files to the cloud & deleting the upload files.
// There must be only one Capture per DataManager. The lifecycle of a Capture is:
//
// - NewSync
// - Reconfigure (any number of times)
// - Close (once).
type Sync struct {
	logger                      logging.Logger
	closedCancelFn              context.CancelFunc
	closedCtx                   context.Context
	clk                         clk.Clock
	flushCollectors             func()
	propagationGoroutineStarted atomic.Bool
	// ConfigPropagated exists only for tests
	ConfigPropagated atomic.Bool
	// FileDeletionBackgroundWorkers temporarily public for tests
	FileDeletionBackgroundWorkers *sync.WaitGroup
	fileDeletionRoutineCancelFn   context.CancelFunc

	mu                        sync.Mutex
	captureDir                string
	cloudConn                 rpc.ClientConn
	cloudConnSvc              cloud.ConnectionService
	datasyncBackgroundWorkers sync.WaitGroup
	// FileLastModifiedMillis temporarily public for tests
	FileLastModifiedMillis int
	filesToSync            chan string
	// MaxSyncThreads temporarily public for tests
	MaxSyncThreads            int
	propagateDataSyncConfigWG sync.WaitGroup
	selectiveSyncEnabled      bool
	syncConfigUpdated         bool
	syncDisabled              bool
	syncIntervalMins          float64
	syncPaths                 []string
	syncRoutineCancelFn       context.CancelFunc
	syncSensor                selectiveSyncer
	// SyncTicker temporarily public for tests
	SyncTicker *clk.Ticker
	// Syncer temporarily public for tests
	Syncer Manager
	// SyncerConstructor temporarily public for tests
	SyncerConstructor            ManagerConstructor
	syncerNeedsToBeReInitialized bool
	tags                         []string
}

// Config is the sync config.
type Config struct {
	AdditionalSyncPaths        []string
	CaptureDir                 string
	CaptureDisabled            bool
	DeleteEveryNthWhenDiskFull int
	FileLastModifiedMillis     int
	MaximumNumSyncThreads      int
	ScheduledSyncDisabled      bool
	SelectiveSyncerName        string
	SyncIntervalMins           float64
	Tags                       []string
}

// Default time to wait in milliseconds to check if a file has been modified.
const defaultFileLastModifiedMillis = 10000.0

// NewSync creates a new Manager.
func NewSync(
	logger logging.Logger,
	clk clk.Clock,
	flushCollectors func(),
) *Sync {
	closedCtx, closedCancelFn := context.WithCancel(context.Background())
	return &Sync{
		flushCollectors:        flushCollectors,
		clk:                    clk,
		closedCtx:              closedCtx,
		closedCancelFn:         closedCancelFn,
		logger:                 logger,
		syncPaths:              []string{},
		tags:                   []string{},
		FileLastModifiedMillis: defaultFileLastModifiedMillis,
		SyncerConstructor:      NewManager,
	}
}

// Reconfigure reconfigures Sync.
func (sm *Sync) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	config resource.Config,
	syncConfig Config,
	cloudConnSvc cloud.ConnectionService,
) {
	if sm.fileDeletionRoutineCancelFn != nil {
		sm.fileDeletionRoutineCancelFn()
	}
	if sm.FileDeletionBackgroundWorkers != nil {
		sm.FileDeletionBackgroundWorkers.Wait()
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.ConfigPropagated.Store(false)
	sm.captureDir = syncConfig.CaptureDir
	// Syncer should be reinitialized if the max sync threads are updated in the config
	newMaxSyncThreadValue := MaxParallelSyncRoutines
	if syncConfig.MaximumNumSyncThreads != 0 {
		newMaxSyncThreadValue = syncConfig.MaximumNumSyncThreads
	}
	sm.syncerNeedsToBeReInitialized = cloudConnSvc != sm.cloudConnSvc || newMaxSyncThreadValue != sm.MaxSyncThreads
	sm.cloudConnSvc = cloudConnSvc

	deleteEveryNthValue := DefaultDeleteEveryNth
	if syncConfig.DeleteEveryNthWhenDiskFull != 0 {
		deleteEveryNthValue = syncConfig.DeleteEveryNthWhenDiskFull
	}

	if syncConfig.CaptureDisabled {
		sm.fileDeletionRoutineCancelFn = nil
		sm.FileDeletionBackgroundWorkers = nil
	}

	sm.syncPaths = append([]string{syncConfig.CaptureDir}, syncConfig.AdditionalSyncPaths...)

	fileLastModifiedMillis := syncConfig.FileLastModifiedMillis
	if fileLastModifiedMillis <= 0 {
		fileLastModifiedMillis = defaultFileLastModifiedMillis
	}

	var (
		syncSensor sensor.Sensor
		err        error
	)

	sm.selectiveSyncEnabled = false
	if syncConfig.SelectiveSyncerName != "" {
		sm.selectiveSyncEnabled = true
		syncSensor, err = sensor.FromDependencies(deps, syncConfig.SelectiveSyncerName)
		if err != nil {
			sm.logger.CErrorw(
				ctx, "unable to initialize selective syncer; will not sync at all until fixed or removed from config", "error", err.Error())
		}
	}
	if sm.syncSensor != syncSensor {
		sm.syncSensor = syncSensor
	}
	syncConfigUpdated := sm.syncDisabled != syncConfig.ScheduledSyncDisabled || sm.syncIntervalMins != syncConfig.SyncIntervalMins ||
		!reflect.DeepEqual(sm.tags, syncConfig.Tags) || sm.FileLastModifiedMillis != fileLastModifiedMillis ||
		sm.MaxSyncThreads != newMaxSyncThreadValue

	if syncConfigUpdated {
		sm.syncConfigUpdated = syncConfigUpdated
		sm.syncDisabled = syncConfig.ScheduledSyncDisabled
		sm.syncIntervalMins = syncConfig.SyncIntervalMins
		sm.tags = syncConfig.Tags
		sm.FileLastModifiedMillis = fileLastModifiedMillis
		sm.MaxSyncThreads = newMaxSyncThreadValue
	}

	// if datacapture is enabled, kick off a go routine to handle disk space filling due to
	// cached datacapture files
	// TODO: Make this use stoppable workers
	if !syncConfig.CaptureDisabled {
		fileDeletionCtx, cancelFunc := context.WithCancel(context.Background())
		sm.fileDeletionRoutineCancelFn = cancelFunc
		sm.FileDeletionBackgroundWorkers = &sync.WaitGroup{}
		sm.FileDeletionBackgroundWorkers.Add(1)
		go deleteExcessFiles(
			fileDeletionCtx,
			sm.FileDeletionBackgroundWorkers,
			syncConfig.CaptureDir,
			deleteEveryNthValue,
			sm.Syncer,
			sm.logger,
		)
	}

	if !sm.propagationGoroutineStarted.Swap(true) {
		sm.startPropagateDataSyncConfig()
	}
}

// readyToSync is a method for getting the bool reading from the selective sync sensor
// for determining whether the key is present and what its value is.
func readyToSync(ctx context.Context, s selectiveSyncer, logger logging.Logger) (readyToSync bool) {
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

func deleteExcessFiles(ctx context.Context, wg *sync.WaitGroup, captureDir string,
	deleteEveryNth int, syncer Manager, logger logging.Logger,
) {
	if runtime.GOOS == "android" {
		logger.Debug("file deletion if disk is full is not currently supported on Android")
		return
	}
	t := DeletionTicker.Ticker(FilesystemPollInterval)
	defer t.Stop()
	defer wg.Done()
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
			shouldDelete, err := ShouldDeleteBasedOnDiskUsage(ctx, captureDir, logger)
			if err != nil {
				logger.Warnw("error checking file system stats", "error", err)
			}
			if shouldDelete {
				start := time.Now()
				deletedFileCount, err := DeleteFiles(ctx, syncer, deleteEveryNth, captureDir, logger)
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

// Close releases all resources managed by data_manager.
func (sm *Sync) Close() {
	sm.closedCancelFn()
	sm.mu.Lock()
	sm.closeSyncer()
	if sm.syncRoutineCancelFn != nil {
		sm.syncRoutineCancelFn()
	}
	if sm.fileDeletionRoutineCancelFn != nil {
		sm.fileDeletionRoutineCancelFn()
	}

	fileDeletionBackgroundWorkers := sm.FileDeletionBackgroundWorkers
	sm.mu.Unlock()
	sm.datasyncBackgroundWorkers.Wait()

	if fileDeletionBackgroundWorkers != nil {
		fileDeletionBackgroundWorkers.Wait()
	}
	sm.propagateDataSyncConfigWG.Wait()
}

func (sm *Sync) closeSyncer() {
	if sm.Syncer != nil {
		// If previously we were syncing, close the old syncer and cancel the old updateCollectors goroutine.
		sm.Syncer.Close()
		close(sm.filesToSync)
		sm.Syncer = nil
	}
	if sm.cloudConn != nil {
		goutils.UncheckedError(sm.cloudConn.Close())
	}
}

var grpcConnectionTimeout = 10 * time.Second

func (sm *Sync) initSyncer(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, grpcConnectionTimeout)
	defer cancel()
	identity, conn, err := sm.cloudConnSvc.AcquireConnection(ctx)
	if err != nil {
		return err
	}

	client := v1.NewDataSyncServiceClient(conn)
	sm.filesToSync = make(chan string)
	sm.Syncer = sm.SyncerConstructor(identity, client, sm.logger, sm.captureDir, sm.MaxSyncThreads, sm.filesToSync)
	sm.cloudConn = conn

	return nil
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
// If automated sync is also enabled, calling Sync will upload the files,
// regardless of whether or not is the scheduled time.
func (sm *Sync) Sync(ctx context.Context, _ map[string]interface{}) error {
	sm.mu.Lock()
	if sm.Syncer == nil {
		err := sm.initSyncer(ctx)
		if err != nil {
			sm.mu.Unlock()
			return err
		}
	}

	sm.mu.Unlock()
	sm.sync(ctx)
	return nil
}

func (sm *Sync) startPropagateDataSyncConfig() {
	sm.propagateDataSyncConfigWG.Add(1)
	goutils.ManagedGo(sm.propagateDataSyncConfigLoop, sm.propagateDataSyncConfigWG.Done)
}

// propagateDataSyncConfigLoop runs until Close() is called on *builtIn
// Immediately on first execution and every second afterwards it
// checks if the datasync configuration has changes which
// have not propagated to datasync.
// If so it propagates the changes and marks the datasync configuration as propagated.
// Otherwise it sleeps for another second.
// Takes the builtIn lock every iteration.
func (sm *Sync) propagateDataSyncConfigLoop() {
	if err := sm.propagateDataSyncConfig(); err != nil {
		return
	}
	for goutils.SelectContextOrWait(sm.closedCtx, time.Millisecond*100) {
		if err := sm.propagateDataSyncConfig(); err != nil {
			return
		}
	}
}

// PropagateDataSyncConfig is temporarily public for tests
// it applies the data sync config set in the previous Reconfigure call.
func (sm *Sync) propagateDataSyncConfig() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if !sm.syncConfigUpdated {
		sm.ConfigPropagated.Store(true)
		return nil
	}
	sm.cancelSyncScheduler()
	enabled := !sm.syncDisabled && sm.syncIntervalMins != 0.0
	if enabled {
		if sm.Syncer == nil {
			if err := sm.initSyncer(sm.closedCtx); err != nil {
				if errors.Is(err, cloud.ErrNotCloudManaged) {
					sm.logger.Debug("Using no-op sync manager when not cloud managed")
					sm.ConfigPropagated.Store(true)
					return err
				}
				sm.logger.Infof("initSyncer err: %s", err.Error())
				return nil
			}
		} else if sm.syncerNeedsToBeReInitialized {
			sm.closeSyncer()
			if err := sm.initSyncer(sm.closedCtx); err != nil {
				if errors.Is(err, cloud.ErrNotCloudManaged) {
					sm.logger.Debug("Using no-op sync manager when not cloud managed")
					sm.ConfigPropagated.Store(true)
					return err
				}
				sm.logger.Infof("initSyncer err: %s", err.Error())
				return nil
			}
		}
		sm.Syncer.SetArbitraryFileTags(sm.tags)
		sm.startSyncScheduler(sm.syncIntervalMins)
	} else {
		if sm.SyncTicker != nil {
			sm.SyncTicker.Stop()
			sm.SyncTicker = nil
		}
		sm.closeSyncer()
	}
	sm.syncConfigUpdated = false
	sm.ConfigPropagated.Store(true)
	return nil
}

// startSyncScheduler starts the goroutine that calls Sync repeatedly if scheduled sync is enabled.
func (sm *Sync) startSyncScheduler(intervalMins float64) {
	cancelCtx, fn := context.WithCancel(sm.closedCtx)
	sm.syncRoutineCancelFn = fn
	sm.uploadData(cancelCtx, intervalMins)
}

// cancelSyncScheduler cancels the goroutine that calls Sync repeatedly if scheduled sync is enabled.
// It does not close the syncer or any in progress uploads.
func (sm *Sync) cancelSyncScheduler() {
	if sm.syncRoutineCancelFn != nil {
		sm.syncRoutineCancelFn()
		sm.syncRoutineCancelFn = nil
		// DATA-2664: A goroutine calling this method must currently be holding the data manager
		// lock. The `uploadData` background goroutine can also acquire the data manager lock prior
		// to learning to exit. Thus we release the lock such that the `uploadData` goroutine can
		// make progress and exit.
		sm.mu.Unlock()
		sm.datasyncBackgroundWorkers.Wait()
		sm.mu.Lock()
	}
}

func (sm *Sync) uploadData(cancelCtx context.Context, intervalMins float64) {
	// time.Duration loses precision at low floating point values, so turn intervalMins to milliseconds.
	intervalMillis := 60000.0 * intervalMins
	// The ticker must be created before uploadData returns to prevent race conditions between clock.Ticker and
	// clock.Add in sync_test.go.
	tkr := sm.clk.Ticker(time.Millisecond * time.Duration(intervalMillis))
	sm.SyncTicker = tkr
	sm.datasyncBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer sm.datasyncBackgroundWorkers.Done()
		defer tkr.Stop()

		for {
			if err := cancelCtx.Err(); err != nil {
				if !errors.Is(err, context.Canceled) {
					sm.logger.Errorw("data manager context closed unexpectedly", "error", err)
				}
				return
			}

			select {
			case <-cancelCtx.Done():
				return
			case <-tkr.C:
				sm.mu.Lock()
				if sm.Syncer != nil {
					// If selective sync is disabled, sync. If it is enabled, check the condition below.
					shouldSync := !sm.selectiveSyncEnabled
					// If selective sync is enabled and the sensor has been properly initialized,
					// try to get the reading from the selective sensor that indicates whether to sync
					if sm.syncSensor != nil && sm.selectiveSyncEnabled {
						shouldSync = readyToSync(cancelCtx, sm.syncSensor, sm.logger)
					}
					sm.mu.Unlock()

					if !isOffline() && shouldSync {
						sm.sync(cancelCtx)
					}
				} else {
					sm.mu.Unlock()
				}
			}
		}
	})
}

func isOffline() bool {
	timeout := 5 * time.Second
	_, err := net.DialTimeout("tcp", "app.viam.com:443", timeout)
	// If there's an error, the system is likely offline.
	return err != nil
}

func (sm *Sync) sync(ctx context.Context) {
	sm.flushCollectors()

	sm.mu.Lock()
	syncer := sm.Syncer
	syncPaths := sm.syncPaths
	fileLastModifiedMillis := sm.FileLastModifiedMillis
	sm.mu.Unlock()

	// Retrieve all files in capture dir and send them to the syncer
	getAllFilesToSync(ctx, syncPaths, fileLastModifiedMillis, syncer, sm.clk)
}

//nolint:errcheck,nilerr
func getAllFilesToSync(ctx context.Context, dirs []string, lastModifiedMillis int, syncer Manager, clock clk.Clock) {
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if ctx.Err() != nil {
				return filepath.SkipAll
			}
			if err != nil {
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
			timeSinceMod := clock.Since(info.ModTime())
			// When using a mock clock in tests, this can be negative since the file system will still use the system clock.
			// Take max(timeSinceMod, 0) to account for this.
			if timeSinceMod < 0 {
				timeSinceMod = 0
			}
			isCompletedCaptureFile := filepath.Ext(path) == datacapture.FileExt
			isNonCaptureFileThatIsNotBeingWrittenTo := filepath.Ext(path) != datacapture.InProgressFileExt &&
				filepath.Ext(path) != datacapture.FileExt &&
				timeSinceMod >= time.Duration(lastModifiedMillis)*time.Millisecond
			if isCompletedCaptureFile || isNonCaptureFileThatIsNotBeingWrittenTo {
				syncer.SendFileToSync(path)
			}
			return nil
		})
	}
}

var (
	// InitialWaitTimeMillis defines the time to wait on the first retried upload attempt.
	InitialWaitTimeMillis = uatomic.NewInt32(1000)
	// RetryExponentialFactor defines the factor by which the retry wait time increases.
	RetryExponentialFactor = uatomic.NewInt32(2)
	// OfflineWaitTimeSeconds defines the amount of time to wait to retry if the machine is offline.
	OfflineWaitTimeSeconds = uatomic.NewInt32(60)
	maxRetryInterval       = 24 * time.Hour
)

// FailedDir is a subdirectory of the capture directory that holds any files that could not be synced.
const FailedDir = "failed"

// MaxParallelSyncRoutines is the maximum number of sync goroutines that can be running at once.
const MaxParallelSyncRoutines = 10

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
	closed     uatomic.Bool
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
