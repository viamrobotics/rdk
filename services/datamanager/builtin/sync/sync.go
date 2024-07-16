// Package sync ....
package sync

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/clock"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/utils"
)

var (
	// temporarily public for tests.
	DefaultDeleteEveryNth = 5
	// temporarily public for tests.
	DeletionTicker = clock.New()
	// temporarily public for tests.
	FilesystemPollInterval = 30 * time.Second
)

type selectiveSyncer interface {
	sensor.Sensor
}
type SyncManager struct {
	logger                      logging.Logger
	closedCancelFn              context.CancelFunc
	closedCtx                   context.Context
	clk                         clock.Clock
	flushCollectors             func()
	propagationGoroutineStarted atomic.Bool

	mu                        sync.Mutex
	captureDir                string
	cloudConn                 rpc.ClientConn
	cloudConnSvc              cloud.ConnectionService
	datasyncBackgroundWorkers sync.WaitGroup
	// temporarily public for tests
	FileDeletionBackgroundWorkers *sync.WaitGroup
	fileDeletionRoutineCancelFn   context.CancelFunc
	// temporarily public for tests
	FileLastModifiedMillis int
	filesToSync            chan string
	// temporarily public for tests
	MaxSyncThreads            int
	propagateDataSyncConfigWG sync.WaitGroup
	selectiveSyncEnabled      bool
	syncConfigUpdated         bool
	syncDisabled              bool
	syncIntervalMins          float64
	syncPaths                 []string
	syncRoutineCancelFn       context.CancelFunc
	syncSensor                selectiveSyncer
	// temporarily public for tests
	SyncTicker *clock.Ticker
	// temporarily public for tests
	Syncer datasync.Manager
	// temporarily public for tests
	SyncerConstructor            datasync.ManagerConstructor
	syncerNeedsToBeReInitialized bool
	tags                         []string
}

// CaptureConfig is the capture manager config.
type SyncConfig struct {
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

func NewSyncManager(
	logger logging.Logger,
	clk clock.Clock,
	flushCollectors func(),
) *SyncManager {
	closedCtx, closedCancelFn := context.WithCancel(context.Background())
	return &SyncManager{
		flushCollectors:        flushCollectors,
		clk:                    clk,
		closedCtx:              closedCtx,
		closedCancelFn:         closedCancelFn,
		logger:                 logger,
		syncPaths:              []string{},
		tags:                   []string{},
		FileLastModifiedMillis: defaultFileLastModifiedMillis,
		SyncerConstructor:      datasync.NewManager,
	}
}

// ReconfigureCapture reconfigures the capture manager.
func (sm *SyncManager) ReconfigureSync(
	ctx context.Context,
	deps resource.Dependencies,
	config resource.Config,
	syncConfig SyncConfig,
) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	cloudConnSvc, err := resource.FromDependencies[cloud.ConnectionService](deps, cloud.InternalServiceName)
	if err != nil {
		return err
	}
	sm.captureDir = syncConfig.CaptureDir
	// Syncer should be reinitialized if the max sync threads are updated in the config
	newMaxSyncThreadValue := datasync.MaxParallelSyncRoutines
	if syncConfig.MaximumNumSyncThreads != 0 {
		newMaxSyncThreadValue = syncConfig.MaximumNumSyncThreads
	}
	sm.syncerNeedsToBeReInitialized = cloudConnSvc != sm.cloudConnSvc || newMaxSyncThreadValue != sm.MaxSyncThreads
	sm.cloudConnSvc = cloudConnSvc

	if sm.fileDeletionRoutineCancelFn != nil {
		sm.fileDeletionRoutineCancelFn()
	}
	if sm.FileDeletionBackgroundWorkers != nil {
		sm.FileDeletionBackgroundWorkers.Wait()
	}
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

	var syncSensor sensor.Sensor
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
	if !syncConfig.CaptureDisabled {
		fileDeletionCtx, cancelFunc := context.WithCancel(context.Background())
		sm.fileDeletionRoutineCancelFn = cancelFunc
		sm.FileDeletionBackgroundWorkers = &sync.WaitGroup{}
		sm.FileDeletionBackgroundWorkers.Add(1)
		go pollFilesystem(fileDeletionCtx, sm.FileDeletionBackgroundWorkers,
			syncConfig.CaptureDir, deleteEveryNthValue, sm.Syncer, sm.logger)
	}

	if !sm.propagationGoroutineStarted.Swap(true) {
		sm.startPropagateDataSyncConfig()
	}
	return nil
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

func pollFilesystem(ctx context.Context, wg *sync.WaitGroup, captureDir string,
	deleteEveryNth int, syncer datasync.Manager, logger logging.Logger,
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
func (sm *SyncManager) Close() error {
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

	return nil
}

func (sm *SyncManager) closeSyncer() {
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

func (sm *SyncManager) initSyncer(ctx context.Context) error {
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
func (sm *SyncManager) Sync(ctx context.Context, _ map[string]interface{}) error {
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

func (sm *SyncManager) startPropagateDataSyncConfig() {
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
func (sm *SyncManager) propagateDataSyncConfigLoop() {
	if err := sm.PropagateDataSyncConfig(); err != nil {
		return
	}
	for goutils.SelectContextOrWait(sm.closedCtx, time.Second) {
		if err := sm.PropagateDataSyncConfig(); err != nil {
			return
		}
	}
}

func (sm *SyncManager) PropagateDataSyncConfig() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if !sm.syncConfigUpdated {
		return nil
	}
	sm.cancelSyncScheduler()
	enabled := !sm.syncDisabled && sm.syncIntervalMins != 0.0
	if enabled {
		if sm.Syncer == nil {
			if err := sm.initSyncer(sm.closedCtx); err != nil {
				if errors.Is(err, cloud.ErrNotCloudManaged) {
					sm.logger.Debug("Using no-op sync manager when not cloud managed")
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
	return nil
}

// startSyncScheduler starts the goroutine that calls Sync repeatedly if scheduled sync is enabled.
func (sm *SyncManager) startSyncScheduler(intervalMins float64) {
	cancelCtx, fn := context.WithCancel(sm.closedCtx)
	sm.syncRoutineCancelFn = fn
	sm.uploadData(cancelCtx, intervalMins)
}

// cancelSyncScheduler cancels the goroutine that calls Sync repeatedly if scheduled sync is enabled.
// It does not close the syncer or any in progress uploads.
func (sm *SyncManager) cancelSyncScheduler() {
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

func (sm *SyncManager) uploadData(cancelCtx context.Context, intervalMins float64) {
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

func (sm *SyncManager) sync(ctx context.Context) {
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
func getAllFilesToSync(ctx context.Context, dirs []string, lastModifiedMillis int, syncer datasync.Manager, clock clock.Clock) {
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if ctx.Err() != nil {
				return filepath.SkipAll
			}
			if err != nil {
				return nil
			}

			// Do not sync the files in the corrupted data directory.
			if info.IsDir() && info.Name() == datasync.FailedDir {
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
