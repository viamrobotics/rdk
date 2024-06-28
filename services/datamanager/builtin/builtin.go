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
	"go.viam.com/rdk/utils"
)

func init() {
	resource.RegisterService(
		datamanager.API,
		resource.DefaultServiceModel,
		resource.Registration[datamanager.Service, *Config]{
			Constructor: NewBuiltIn,
			WeakDependencies: []resource.Matcher{
				resource.TypeMatcher{Type: resource.APITypeComponentName},
				resource.SubtypeMatcher{Subtype: slam.SubtypeName},
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
	CaptureDir                  string   `json:"capture_dir"`
	AdditionalSyncPaths         []string `json:"additional_sync_paths"`
	SyncIntervalMins            float64  `json:"sync_interval_mins"`
	CaptureDisabled             bool     `json:"capture_disabled"`
	ScheduledSyncDisabled       bool     `json:"sync_disabled"`
	Tags                        []string `json:"tags"`
	FileLastModifiedMillis      int      `json:"file_last_modified_millis"`
	SelectiveSyncerName         string   `json:"selective_syncer_name"`
	MaximumNumSyncThreads       int      `json:"maximum_num_sync_threads"`
	DeleteEveryNthWhenDiskFull  int      `json:"delete_every_nth_when_disk_full"`
	MaximumCaptureFileSizeBytes int64    `json:"maximum_capture_file_size_bytes"`
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
	resource.Named
	logger                 logging.Logger
	lock                   sync.Mutex
	backgroundWorkers      sync.WaitGroup
	fileLastModifiedMillis int

	// Dan: This now includes the capture dir. Change to syncPaths
	additionalSyncPaths []string
	tags                []string
	syncDisabled        bool
	syncIntervalMins    float64
	syncRoutineCancelFn context.CancelFunc
	syncer              datasync.Manager
	syncerConstructor   datasync.ManagerConstructor
	filesToSync         chan string
	maxSyncThreads      int
	cloudConnSvc        cloud.ConnectionService
	cloudConn           rpc.ClientConn
	syncTicker          *clk.Ticker

	syncSensor           selectiveSyncer
	selectiveSyncEnabled bool

	fileDeletionRoutineCancelFn   context.CancelFunc
	fileDeletionBackgroundWorkers *sync.WaitGroup

	captureManager *data.CaptureManager
}

// NewBuiltIn returns a new data manager service for the given robot.
func NewBuiltIn(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (datamanager.Service, error) {
	svc := &builtIn{
		Named:                  conf.ResourceName().AsNamed(),
		logger:                 logger,
		syncIntervalMins:       0,
		additionalSyncPaths:    []string{},
		tags:                   []string{},
		fileLastModifiedMillis: defaultFileLastModifiedMillis,
		syncerConstructor:      datasync.NewManager,
		selectiveSyncEnabled:   false,
		captureManager:         data.NewCaptureManager(logger.Sublogger("capture"), clock),
	}

	if err := svc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return svc, nil
}

// Close releases all resources managed by data_manager.
func (svc *builtIn) Close(_ context.Context) error {
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
	svc.backgroundWorkers.Wait()

	if fileDeletionBackgroundWorkers != nil {
		fileDeletionBackgroundWorkers.Wait()
	}

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
	if errors.Is(err, cloud.ErrNotCloudManaged) {
		svc.logger.CDebug(ctx, "Using no-op sync manager when not cloud managed")
		svc.syncer = datasync.NewNoopManager()
	}
	if err != nil {
		return err
	}

	client := v1.NewDataSyncServiceClient(conn)
	svc.filesToSync = make(chan string)
	syncer, err := svc.syncerConstructor(identity, client, svc.logger, svc.captureManager.CaptureDir(), svc.maxSyncThreads, svc.filesToSync)
	if err != nil {
		return errors.Wrap(err, "failed to initialize new syncer")
	}
	svc.syncer = syncer
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

	dataConfig := data.Config{
		CaptureDisabled:             svcConfig.CaptureDisabled,
		CaptureDir:                  svcConfig.CaptureDir,
		Tags:                        svcConfig.Tags,
		MaximumCaptureFileSizeBytes: svcConfig.MaximumCaptureFileSizeBytes,
	}
	if err = svc.captureManager.Reconfigure(ctx, deps, conf, dataConfig); err != nil {
		svc.logger.Warnw("DataCapture reconfigure error", "err", err)
		return err
	}

	// Syncer should be reinitialized if the max sync threads are updated in the config
	newMaxSyncThreadValue := datasync.MaxParallelSyncRoutines
	if svcConfig.MaximumNumSyncThreads != 0 {
		newMaxSyncThreadValue = svcConfig.MaximumNumSyncThreads
	}
	reinitSyncer := cloudConnSvc != svc.cloudConnSvc || newMaxSyncThreadValue != svc.maxSyncThreads
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

	svc.additionalSyncPaths = append([]string{svc.captureManager.CaptureDir()}, svcConfig.AdditionalSyncPaths...)

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
		svc.syncDisabled = svcConfig.ScheduledSyncDisabled
		svc.syncIntervalMins = svcConfig.SyncIntervalMins
		svc.tags = svcConfig.Tags
		svc.fileLastModifiedMillis = fileLastModifiedMillis
		svc.maxSyncThreads = newMaxSyncThreadValue

		svc.cancelSyncScheduler()
		if !svc.syncDisabled && svc.syncIntervalMins != 0.0 {
			if svc.syncer == nil {
				if err := svc.initSyncer(ctx); err != nil {
					return err
				}
			} else if reinitSyncer {
				svc.closeSyncer()
				if err := svc.initSyncer(ctx); err != nil {
					return err
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
	}

	// if datacapture is enabled, kick off a go routine to check if disk space is filling due to
	// cached datacapture files
	if !svcConfig.CaptureDisabled {
		fileDeletionCtx, cancelFunc := context.WithCancel(context.Background())
		svc.fileDeletionRoutineCancelFn = cancelFunc
		svc.fileDeletionBackgroundWorkers = &sync.WaitGroup{}
		svc.fileDeletionBackgroundWorkers.Add(1)
		go pollFilesystem(fileDeletionCtx, svc.fileDeletionBackgroundWorkers,
			svc.captureManager.CaptureDir(), deleteEveryNthValue, svc.syncer, svc.logger)
	}

	return nil
}

// startSyncScheduler starts the goroutine that calls Sync repeatedly if scheduled sync is enabled.
func (svc *builtIn) startSyncScheduler(intervalMins float64) {
	cancelCtx, fn := context.WithCancel(context.Background())
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
		svc.backgroundWorkers.Wait()
		svc.lock.Lock()
	}
}

func (svc *builtIn) uploadData(cancelCtx context.Context, intervalMins float64) {
	// time.Duration loses precision at low floating point values, so turn intervalMins to milliseconds.
	intervalMillis := 60000.0 * intervalMins
	// The ticker must be created before uploadData returns to prevent race conditions between clock.Ticker and
	// clock.Add in sync_test.go.
	svc.syncTicker = clock.Ticker(time.Millisecond * time.Duration(intervalMillis))
	svc.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer svc.backgroundWorkers.Done()
		defer svc.syncTicker.Stop()

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
			case <-svc.syncTicker.C:
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
	syncPaths := svc.additionalSyncPaths
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
			isStuckInProgressCaptureFile := filepath.Ext(path) == datacapture.InProgressFileExt &&
				timeSinceMod >= defaultFileLastModifiedMillis*time.Millisecond
			isNonCaptureFileThatIsNotBeingWrittenTo := filepath.Ext(path) != datacapture.InProgressFileExt &&
				timeSinceMod >= time.Duration(lastModifiedMillis)*time.Millisecond
			isCompletedCaptureFile := filepath.Ext(path) == datacapture.FileExt
			if isCompletedCaptureFile || isStuckInProgressCaptureFile || isNonCaptureFileThatIsNotBeingWrittenTo {
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
