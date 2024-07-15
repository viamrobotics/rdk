// Package builtin contains a service type that can be used to capture data from a robot's components.
package builtin

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
)

// In order for a collector to be captured by Data Capture, it must be included as a weak dependency.
func init() {
	resource.RegisterService(
		datamanager.API,
		resource.DefaultServiceModel,
		resource.Registration[datamanager.Service, *Config]{
			Constructor: NewBuiltIn,
			WeakDependencies: []resource.Matcher{
				resource.TypeMatcher{Type: resource.APITypeComponentName},
				resource.SubtypeMatcher{Subtype: slam.SubtypeName},
				resource.SubtypeMatcher{Subtype: vision.SubtypeName},
			},
		})
}

// Default time to wait in milliseconds to check if a file has been modified.
const defaultFileLastModifiedMillis = 10000.0

// Default time between disk size checks.
var filesystemPollInterval = 30 * time.Second

var (
	clock          = clk.New()
	deletionTicker = clk.New()
)

// Config describes how to configure the service.
type Config struct {
	// Sync & Capture
	CaptureDir string   `json:"capture_dir"`
	Tags       []string `json:"tags"`
	// Capture
	CaptureDisabled             bool  `json:"capture_disabled"`
	DeleteEveryNthWhenDiskFull  int   `json:"delete_every_nth_when_disk_full"`
	MaximumCaptureFileSizeBytes int64 `json:"maximum_capture_file_size_bytes"`
	// Sync
	AdditionalSyncPaths    []string `json:"additional_sync_paths"`
	FileLastModifiedMillis int      `json:"file_last_modified_millis"`
	MaximumNumSyncThreads  int      `json:"maximum_num_sync_threads"`
	ScheduledSyncDisabled  bool     `json:"sync_disabled"`
	SelectiveSyncerName    string   `json:"selective_syncer_name"`
	SyncIntervalMins       float64  `json:"sync_interval_mins"`
}

// Validate returns components which will be depended upon weakly due to the above matcher.
func (c *Config) Validate(path string) ([]string, error) {
	return []string{cloud.InternalServiceName.String()}, nil
}

type selectiveSyncer interface {
	sensor.Sensor
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

// builtIn initializes and orchestrates data capture collectors for registered component/methods.
type builtIn struct {
	// Live for lifetime of builtIn
	resource.Named
	logger         logging.Logger
	closedCancelFn context.CancelFunc
	closedCtx      context.Context
	lock           sync.Mutex

	// Capture
	captureManager *data.CaptureManager

	// Sync
	cloudConn                     rpc.ClientConn
	cloudConnSvc                  cloud.ConnectionService
	datasyncBackgroundWorkers     sync.WaitGroup
	fileDeletionBackgroundWorkers *sync.WaitGroup
	fileDeletionRoutineCancelFn   context.CancelFunc
	fileLastModifiedMillis        int
	filesToSync                   chan string
	maxSyncThreads                int
	propagateDataSyncConfigWG     sync.WaitGroup
	selectiveSyncEnabled          bool
	syncConfigUpdated             bool
	syncDisabled                  bool
	syncIntervalMins              float64
	syncPaths                     []string
	syncRoutineCancelFn           context.CancelFunc
	syncSensor                    selectiveSyncer
	syncTicker                    *clk.Ticker
	syncer                        datasync.Manager
	syncerConstructor             datasync.ManagerConstructor
	syncerNeedsToBeReInitialized  bool
	tags                          []string
}

// NewBuiltIn returns a new data manager service for the given robot.
func NewBuiltIn(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (datamanager.Service, error) {
	closedCtx, closedCancelFn := context.WithCancel(context.Background())
	svc := &builtIn{
		closedCtx:              closedCtx,
		closedCancelFn:         closedCancelFn,
		Named:                  conf.ResourceName().AsNamed(),
		logger:                 logger,
		syncPaths:              []string{},
		tags:                   []string{},
		fileLastModifiedMillis: defaultFileLastModifiedMillis,
		syncerConstructor:      datasync.NewManager,
		captureManager:         data.NewCaptureManager(logger.Sublogger("capture"), clock),
	}

	if err := svc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	svc.startPropagateDataSyncConfig()
	return svc, nil
}

// Close releases all resources managed by data_manager.
func (svc *builtIn) Close(_ context.Context) error {
	svc.closedCancelFn()
	svc.lock.Lock()
	svc.closeSyncer()
	if svc.syncRoutineCancelFn != nil {
		svc.syncRoutineCancelFn()
	}
	if svc.fileDeletionRoutineCancelFn != nil {
		svc.fileDeletionRoutineCancelFn()
	}

	fileDeletionBackgroundWorkers := svc.fileDeletionBackgroundWorkers
	svc.lock.Unlock()
	svc.captureManager.Close()
	svc.datasyncBackgroundWorkers.Wait()

	if fileDeletionBackgroundWorkers != nil {
		fileDeletionBackgroundWorkers.Wait()
	}
	svc.propagateDataSyncConfigWG.Wait()

	return nil
}

func (svc *builtIn) closeSyncer() {
	if svc.syncer != nil {
		// If previously we were syncing, close the old syncer and cancel the old updateCollectors goroutine.
		svc.syncer.Close()
		close(svc.filesToSync)
		svc.syncer = nil
	}
	if svc.cloudConn != nil {
		goutils.UncheckedError(svc.cloudConn.Close())
	}
}

var grpcConnectionTimeout = 10 * time.Second

func (svc *builtIn) initSyncer(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, grpcConnectionTimeout)
	defer cancel()
	identity, conn, err := svc.cloudConnSvc.AcquireConnection(ctx)
	if err != nil {
		return err
	}

	client := v1.NewDataSyncServiceClient(conn)
	svc.filesToSync = make(chan string)
	svc.syncer = svc.syncerConstructor(identity, client, svc.logger, svc.captureManager.CaptureDir(), svc.maxSyncThreads, svc.filesToSync)
	svc.cloudConn = conn

	return nil
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
// If automated sync is also enabled, calling Sync will upload the files,
// regardless of whether or not is the scheduled time.
func (svc *builtIn) Sync(ctx context.Context, _ map[string]interface{}) error {
	svc.lock.Lock()
	if svc.syncer == nil {
		err := svc.initSyncer(ctx)
		if err != nil {
			svc.lock.Unlock()
			return err
		}
	}

	svc.lock.Unlock()
	svc.sync(ctx)
	return nil
}

// Reconfigure updates the data manager service when the config has changed.
func (svc *builtIn) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	g := utils.NewGuard(func() { goutils.UncheckedError(svc.Close(ctx)) })
	defer g.OnFail()
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svcConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	cloudConnSvc, err := resource.FromDependencies[cloud.ConnectionService](deps, cloud.InternalServiceName)
	if err != nil {
		return err
	}

	captureConfig := data.CaptureConfig{
		CaptureDisabled:             svcConfig.CaptureDisabled,
		CaptureDir:                  svcConfig.CaptureDir,
		Tags:                        svcConfig.Tags,
		MaximumCaptureFileSizeBytes: svcConfig.MaximumCaptureFileSizeBytes,
	}
	if err = svc.captureManager.ReconfigureCapture(ctx, deps, conf, captureConfig); err != nil {
		svc.logger.Warnw("DataCapture reconfigure error", "err", err)
		return err
	}

	// Syncer should be reinitialized if the max sync threads are updated in the config
	newMaxSyncThreadValue := datasync.MaxParallelSyncRoutines
	if svcConfig.MaximumNumSyncThreads != 0 {
		newMaxSyncThreadValue = svcConfig.MaximumNumSyncThreads
	}
	svc.syncerNeedsToBeReInitialized = cloudConnSvc != svc.cloudConnSvc || newMaxSyncThreadValue != svc.maxSyncThreads
	svc.cloudConnSvc = cloudConnSvc

	if svc.fileDeletionRoutineCancelFn != nil {
		svc.fileDeletionRoutineCancelFn()
	}
	if svc.fileDeletionBackgroundWorkers != nil {
		svc.fileDeletionBackgroundWorkers.Wait()
	}
	deleteEveryNthValue := defaultDeleteEveryNth
	if svcConfig.DeleteEveryNthWhenDiskFull != 0 {
		deleteEveryNthValue = svcConfig.DeleteEveryNthWhenDiskFull
	}

	if svcConfig.CaptureDisabled {
		svc.fileDeletionRoutineCancelFn = nil
		svc.fileDeletionBackgroundWorkers = nil
	}

	svc.syncPaths = append([]string{svc.captureManager.CaptureDir()}, svcConfig.AdditionalSyncPaths...)

	fileLastModifiedMillis := svcConfig.FileLastModifiedMillis
	if fileLastModifiedMillis <= 0 {
		fileLastModifiedMillis = defaultFileLastModifiedMillis
	}

	var syncSensor sensor.Sensor
	if svcConfig.SelectiveSyncerName != "" {
		svc.selectiveSyncEnabled = true
		syncSensor, err = sensor.FromDependencies(deps, svcConfig.SelectiveSyncerName)
		if err != nil {
			svc.logger.CErrorw(
				ctx, "unable to initialize selective syncer; will not sync at all until fixed or removed from config", "error", err.Error())
		}
	} else {
		svc.selectiveSyncEnabled = false
	}
	if svc.syncSensor != syncSensor {
		svc.syncSensor = syncSensor
	}
	syncConfigUpdated := svc.syncDisabled != svcConfig.ScheduledSyncDisabled || svc.syncIntervalMins != svcConfig.SyncIntervalMins ||
		!reflect.DeepEqual(svc.tags, svcConfig.Tags) || svc.fileLastModifiedMillis != fileLastModifiedMillis ||
		svc.maxSyncThreads != newMaxSyncThreadValue

	if syncConfigUpdated {
		svc.syncConfigUpdated = syncConfigUpdated
		svc.syncDisabled = svcConfig.ScheduledSyncDisabled
		svc.syncIntervalMins = svcConfig.SyncIntervalMins
		svc.tags = svcConfig.Tags
		svc.fileLastModifiedMillis = fileLastModifiedMillis
		svc.maxSyncThreads = newMaxSyncThreadValue
	}

	// if datacapture is enabled, kick off a go routine to handle disk space filling due to
	// cached datacapture files
	if !svcConfig.CaptureDisabled {
		fileDeletionCtx, cancelFunc := context.WithCancel(context.Background())
		svc.fileDeletionRoutineCancelFn = cancelFunc
		svc.fileDeletionBackgroundWorkers = &sync.WaitGroup{}
		svc.fileDeletionBackgroundWorkers.Add(1)
		go pollFilesystem(fileDeletionCtx, svc.fileDeletionBackgroundWorkers,
			svc.captureManager.CaptureDir(), deleteEveryNthValue, svc.syncer, svc.logger)
	}

	g.Success()
	return nil
}

func (svc *builtIn) startPropagateDataSyncConfig() {
	svc.propagateDataSyncConfigWG.Add(1)
	goutils.ManagedGo(svc.propagateDataSyncConfigLoop, svc.propagateDataSyncConfigWG.Done)
}

// propagateDataSyncConfigLoop runs until Close() is called on *builtIn
// Immediately on first execution and every second afterwards it
// checks if the datasync configuration has changes which
// have not propagated to datasync.
// If so it propagates the changes and marks the datasync configuration as propagated.
// Otherwise it sleeps for another second.
// Takes the builtIn lock every iteration.
func (svc *builtIn) propagateDataSyncConfigLoop() {
	if err := svc.propagateDataSyncConfig(); err != nil {
		return
	}
	for goutils.SelectContextOrWait(svc.closedCtx, time.Second) {
		if err := svc.propagateDataSyncConfig(); err != nil {
			return
		}
	}
}

func (svc *builtIn) propagateDataSyncConfig() error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if !svc.syncConfigUpdated {
		return nil
	}
	svc.cancelSyncScheduler()
	enabled := !svc.syncDisabled && svc.syncIntervalMins != 0.0
	if enabled {
		if svc.syncer == nil {
			if err := svc.initSyncer(svc.closedCtx); err != nil {
				if errors.Is(err, cloud.ErrNotCloudManaged) {
					svc.logger.Debug("Using no-op sync manager when not cloud managed")
					return err
				}
				svc.logger.Infof("initSyncer err: %s", err.Error())
				return nil
			}
		} else if svc.syncerNeedsToBeReInitialized {
			svc.closeSyncer()
			if err := svc.initSyncer(svc.closedCtx); err != nil {
				if errors.Is(err, cloud.ErrNotCloudManaged) {
					svc.logger.Debug("Using no-op sync manager when not cloud managed")
					return err
				}
				svc.logger.Infof("initSyncer err: %s", err.Error())
				return nil
			}
		}
		svc.syncer.SetArbitraryFileTags(svc.tags)
		svc.startSyncScheduler(svc.syncIntervalMins)
	} else {
		if svc.syncTicker != nil {
			svc.syncTicker.Stop()
			svc.syncTicker = nil
		}
		svc.closeSyncer()
	}
	svc.syncConfigUpdated = false
	return nil
}

// startSyncScheduler starts the goroutine that calls Sync repeatedly if scheduled sync is enabled.
func (svc *builtIn) startSyncScheduler(intervalMins float64) {
	cancelCtx, fn := context.WithCancel(svc.closedCtx)
	svc.syncRoutineCancelFn = fn
	svc.uploadData(cancelCtx, intervalMins)
}

// cancelSyncScheduler cancels the goroutine that calls Sync repeatedly if scheduled sync is enabled.
// It does not close the syncer or any in progress uploads.
func (svc *builtIn) cancelSyncScheduler() {
	if svc.syncRoutineCancelFn != nil {
		svc.syncRoutineCancelFn()
		svc.syncRoutineCancelFn = nil
		// DATA-2664: A goroutine calling this method must currently be holding the data manager
		// lock. The `uploadData` background goroutine can also acquire the data manager lock prior
		// to learning to exit. Thus we release the lock such that the `uploadData` goroutine can
		// make progress and exit.
		svc.lock.Unlock()
		svc.datasyncBackgroundWorkers.Wait()
		svc.lock.Lock()
	}
}

func (svc *builtIn) uploadData(cancelCtx context.Context, intervalMins float64) {
	// time.Duration loses precision at low floating point values, so turn intervalMins to milliseconds.
	intervalMillis := 60000.0 * intervalMins
	// The ticker must be created before uploadData returns to prevent race conditions between clock.Ticker and
	// clock.Add in sync_test.go.
	tkr := clock.Ticker(time.Millisecond * time.Duration(intervalMillis))
	svc.syncTicker = tkr
	svc.datasyncBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer svc.datasyncBackgroundWorkers.Done()
		defer tkr.Stop()

		for {
			if err := cancelCtx.Err(); err != nil {
				if !errors.Is(err, context.Canceled) {
					svc.logger.Errorw("data manager context closed unexpectedly", "error", err)
				}
				return
			}

			select {
			case <-cancelCtx.Done():
				return
			case <-tkr.C:
				svc.lock.Lock()
				if svc.syncer != nil {
					// If selective sync is disabled, sync. If it is enabled, check the condition below.
					shouldSync := !svc.selectiveSyncEnabled
					// If selective sync is enabled and the sensor has been properly initialized,
					// try to get the reading from the selective sensor that indicates whether to sync
					if svc.syncSensor != nil && svc.selectiveSyncEnabled {
						shouldSync = readyToSync(cancelCtx, svc.syncSensor, svc.logger)
					}
					svc.lock.Unlock()

					if !isOffline() && shouldSync {
						svc.sync(cancelCtx)
					}
				} else {
					svc.lock.Unlock()
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

func (svc *builtIn) sync(ctx context.Context) {
	svc.captureManager.FlushCollectors()

	svc.lock.Lock()
	syncer := svc.syncer
	syncPaths := svc.syncPaths
	fileLastModifiedMillis := svc.fileLastModifiedMillis
	svc.lock.Unlock()

	// Retrieve all files in capture dir and send them to the syncer
	getAllFilesToSync(ctx, syncPaths, fileLastModifiedMillis, syncer)
}

//nolint:errcheck,nilerr
func getAllFilesToSync(ctx context.Context, dirs []string, lastModifiedMillis int, syncer datasync.Manager) {
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

func pollFilesystem(ctx context.Context, wg *sync.WaitGroup, captureDir string,
	deleteEveryNth int, syncer datasync.Manager, logger logging.Logger,
) {
	if runtime.GOOS == "android" {
		logger.Debug("file deletion if disk is full is not currently supported on Android")
		return
	}
	t := deletionTicker.Ticker(filesystemPollInterval)
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
			shouldDelete, err := shouldDeleteBasedOnDiskUsage(ctx, captureDir, logger)
			if err != nil {
				logger.Warnw("error checking file system stats", "error", err)
			}
			if shouldDelete {
				start := time.Now()
				deletedFileCount, err := deleteFiles(ctx, syncer, deleteEveryNth, captureDir, logger)
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
