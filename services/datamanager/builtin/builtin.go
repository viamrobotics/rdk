// Package builtin contains a service type that can be used to capture data from a robot's components.
package builtin

import (
	"context"
	"fmt"
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
	"go.viam.com/rdk/protoutils"
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

// TODO: re-determine if queue size is optimal given we now support 10khz+ capture rates
// The Collector's queue should be big enough to ensure that .capture() is never blocked by the queue being
// written to disk. A default value of 250 was chosen because even with the fastest reasonable capture interval (1ms),
// this would leave 250ms for a (buffered) disk write before blocking, which seems sufficient for the size of
// writes this would be performing.
const defaultCaptureQueueSize = 250

// Default bufio.Writer buffer size in bytes.
const defaultCaptureBufferSize = 4096

// Default time to wait in milliseconds to check if a file has been modified.
const defaultFileLastModifiedMillis = 10000.0

// Default maximum size in bytes of a data capture file.
var defaultMaxCaptureSize = int64(256 * 1024)

// Default time between disk size checks.
var filesystemPollInterval = 30 * time.Second

var (
	clock          = clk.New()
	deletionTicker = clk.New()
)

var errCaptureDirectoryConfigurationDisabled = errors.New("changing the capture directory is prohibited in this environment")

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
	captureDir             string
	captureDisabled        bool
	collectors             map[resourceMethodMetadata]*collectorAndConfig
	lock                   sync.Mutex
	backgroundWorkers      sync.WaitGroup
	fileLastModifiedMillis int

	additionalSyncPaths []string
	tags                []string
	syncDisabled        bool
	syncIntervalMins    float64
	syncRoutineCancelFn context.CancelFunc
	syncer              datasync.Manager
	syncerConstructor   datasync.ManagerConstructor
	maxSyncThreads      int
	cloudConnSvc        cloud.ConnectionService
	cloudConn           rpc.ClientConn
	syncTicker          *clk.Ticker
	maxCaptureFileSize  int64

	syncSensor           selectiveSyncer
	selectiveSyncEnabled bool

	componentMethodFrequencyHz map[resourceMethodMetadata]float32

	fileDeletionRoutineCancelFn   context.CancelFunc
	fileDeletionBackgroundWorkers *sync.WaitGroup
}

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), ".viam", "capture")

// NewBuiltIn returns a new data manager service for the given robot.
func NewBuiltIn(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (datamanager.Service, error) {
	svc := &builtIn{
		Named:                      conf.ResourceName().AsNamed(),
		logger:                     logger,
		captureDir:                 viamCaptureDotDir,
		collectors:                 make(map[resourceMethodMetadata]*collectorAndConfig),
		syncIntervalMins:           0,
		additionalSyncPaths:        []string{},
		tags:                       []string{},
		fileLastModifiedMillis:     defaultFileLastModifiedMillis,
		syncerConstructor:          datasync.NewManager,
		selectiveSyncEnabled:       false,
		componentMethodFrequencyHz: make(map[resourceMethodMetadata]float32),
	}

	if err := svc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return svc, nil
}

// Close releases all resources managed by data_manager.
func (svc *builtIn) Close(_ context.Context) error {
	svc.lock.Lock()
	svc.closeCollectors()
	svc.closeSyncer()
	if svc.syncRoutineCancelFn != nil {
		svc.syncRoutineCancelFn()
	}
	if svc.fileDeletionRoutineCancelFn != nil {
		svc.fileDeletionRoutineCancelFn()
	}

	svc.lock.Unlock()
	svc.backgroundWorkers.Wait()

	if svc.fileDeletionBackgroundWorkers != nil {
		svc.fileDeletionBackgroundWorkers.Wait()
	}

	return nil
}

func (svc *builtIn) closeCollectors() {
	var wg sync.WaitGroup
	for md, collector := range svc.collectors {
		currCollector := collector
		wg.Add(1)
		go func() {
			defer wg.Done()
			currCollector.Collector.Close()
		}()
		delete(svc.collectors, md)
	}
	wg.Wait()
}

func (svc *builtIn) flushCollectors() {
	var wg sync.WaitGroup
	for _, collector := range svc.collectors {
		currCollector := collector
		wg.Add(1)
		go func() {
			defer wg.Done()
			currCollector.Collector.Flush()
		}()
	}
	wg.Wait()
}

// Parameters stored for each collector.
type collectorAndConfig struct {
	Resource  resource.Resource
	Collector data.Collector
	Config    datamanager.DataCaptureConfig
}

// Identifier for a particular collector: component name, component model, component type,
// method parameters, and method name.
type resourceMethodMetadata struct {
	ResourceName   string
	MethodParams   string
	MethodMetadata data.MethodMetadata
}

func (r resourceMethodMetadata) String() string {
	return fmt.Sprintf(
		"[API: %s, Resource Name: %s, Method Name: %s, Method Params: %s]",
		r.MethodMetadata.API, r.ResourceName, r.MethodMetadata.MethodName, r.MethodParams)
}

// Get time.Duration from hz.
func getDurationFromHz(captureFrequencyHz float32) time.Duration {
	if captureFrequencyHz == 0 {
		return time.Duration(0)
	}
	return time.Duration(float32(time.Second) / captureFrequencyHz)
}

var metadataToAdditionalParamFields = map[string]string{
	generateMetadataKey("rdk:component:board", "Analogs"): "reader_name",
	generateMetadataKey("rdk:component:board", "Gpios"):   "pin_name",
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (svc *builtIn) initializeOrUpdateCollector(
	res resource.Resource,
	md resourceMethodMetadata,
	config datamanager.DataCaptureConfig,
	maxFileSizeChanged bool,
) (*collectorAndConfig, error) {
	// Build metadata.
	captureMetadata, err := datacapture.BuildCaptureMetadata(
		config.Name.API,
		config.Name.ShortName(),
		config.Method,
		config.AdditionalParams,
		config.Tags,
	)
	if err != nil {
		return nil, err
	}

	// TODO(DATA-451): validate method params

	if storedCollectorAndConfig, ok := svc.collectors[md]; ok {
		if storedCollectorAndConfig.Config.Equals(&config) &&
			res == storedCollectorAndConfig.Resource &&
			!maxFileSizeChanged {
			// If the attributes have not changed, do nothing and leave the existing collector.
			return svc.collectors[md], nil
		}
		// If the attributes have changed, close the existing collector.
		storedCollectorAndConfig.Collector.Close()
	}

	// Get collector constructor for the component API and method.
	collectorConstructor := data.CollectorLookup(md.MethodMetadata)
	if collectorConstructor == nil {
		return nil, errors.Errorf("failed to find collector constructor for %s", md.MethodMetadata)
	}

	// Parameters to initialize collector.
	interval := getDurationFromHz(config.CaptureFrequencyHz)
	// Set queue size to defaultCaptureQueueSize if it was not set in the config.
	captureQueueSize := config.CaptureQueueSize
	if captureQueueSize == 0 {
		captureQueueSize = defaultCaptureQueueSize
	}

	captureBufferSize := config.CaptureBufferSize
	if captureBufferSize == 0 {
		captureBufferSize = defaultCaptureBufferSize
	}
	additionalParamKey, ok := metadataToAdditionalParamFields[generateMetadataKey(
		md.MethodMetadata.API.String(),
		md.MethodMetadata.MethodName)]
	if ok {
		if _, ok := config.AdditionalParams[additionalParamKey]; !ok {
			return nil, errors.Errorf("failed to validate additional_params for %s, must supply %s",
				md.MethodMetadata.API.String(), additionalParamKey)
		}
	}
	methodParams, err := protoutils.ConvertStringMapToAnyPBMap(config.AdditionalParams)
	if err != nil {
		return nil, err
	}

	// Create a collector for this resource and method.
	targetDir := datacapture.FilePathWithReplacedReservedChars(
		filepath.Join(svc.captureDir, captureMetadata.GetComponentType(),
			captureMetadata.GetComponentName(), captureMetadata.GetMethodName()))
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, err
	}
	params := data.CollectorParams{
		ComponentName: config.Name.ShortName(),
		Interval:      interval,
		MethodParams:  methodParams,
		Target:        datacapture.NewBuffer(targetDir, captureMetadata, svc.maxCaptureFileSize),
		QueueSize:     captureQueueSize,
		BufferSize:    captureBufferSize,
		Logger:        svc.logger,
		Clock:         clock,
	}
	collector, err := (*collectorConstructor)(res, params)
	if err != nil {
		return nil, err
	}
	collector.Collect()

	return &collectorAndConfig{res, collector, config}, nil
}

func (svc *builtIn) closeSyncer() {
	if svc.syncer != nil {
		// If previously we were syncing, close the old syncer and cancel the old updateCollectors goroutine.
		svc.syncer.Close()
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
	syncer, err := svc.syncerConstructor(identity, client, svc.logger, svc.captureDir, svc.maxSyncThreads)
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
	svc.sync()
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

	// Syncer should be reinitialized if the max sync threads are updated in the config
	newMaxSyncThreadValue := datasync.MaxParallelSyncRoutines
	if svcConfig.MaximumNumSyncThreads != 0 {
		newMaxSyncThreadValue = svcConfig.MaximumNumSyncThreads
	}
	reinitSyncer := cloudConnSvc != svc.cloudConnSvc || newMaxSyncThreadValue != svc.maxSyncThreads
	svc.cloudConnSvc = cloudConnSvc

	captureConfigs, err := svc.updateDataCaptureConfigs(deps, conf, svcConfig.CaptureDir)
	if err != nil {
		return err
	}

	if !utils.IsTrustedEnvironment(ctx) && svcConfig.CaptureDir != "" && svcConfig.CaptureDir != viamCaptureDotDir {
		return errCaptureDirectoryConfigurationDisabled
	}

	if svcConfig.CaptureDir != "" {
		svc.captureDir = svcConfig.CaptureDir
	} else {
		svc.captureDir = viamCaptureDotDir
	}
	svc.captureDisabled = svcConfig.CaptureDisabled
	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if svc.captureDisabled {
		svc.closeCollectors()
		svc.collectors = make(map[resourceMethodMetadata]*collectorAndConfig)
	}

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

	// Initialize or add collectors based on changes to the component configurations.
	newCollectors := make(map[resourceMethodMetadata]*collectorAndConfig)
	if !svc.captureDisabled {
		for res, resConfs := range captureConfigs {
			for _, resConf := range resConfs {
				if resConf.Method == "" {
					continue
				}
				// Create component/method metadata
				methodMetadata := data.MethodMetadata{
					API:        resConf.Name.API,
					MethodName: resConf.Method,
				}

				componentMethodMetadata := resourceMethodMetadata{
					ResourceName:   resConf.Name.ShortName(),
					MethodMetadata: methodMetadata,
					MethodParams:   fmt.Sprintf("%v", resConf.AdditionalParams),
				}
				_, ok := svc.componentMethodFrequencyHz[componentMethodMetadata]

				// Only log capture frequency if the component frequency is new or the frequency has changed
				// otherwise we'll be logging way too much
				if !ok || (ok && resConf.CaptureFrequencyHz != svc.componentMethodFrequencyHz[componentMethodMetadata]) {
					syncVal := "will"
					if resConf.CaptureFrequencyHz == 0 {
						syncVal += " not"
					}
					svc.logger.Infof(
						"capture frequency for %s is set to %.2fHz and %s sync", componentMethodMetadata, resConf.CaptureFrequencyHz, syncVal,
					)
				}

				// we need this map to keep track of if state has changed in the configs
				// without it, we will be logging the same message over and over for no reason
				svc.componentMethodFrequencyHz[componentMethodMetadata] = resConf.CaptureFrequencyHz

				maxCaptureFileSize := svcConfig.MaximumCaptureFileSizeBytes
				if maxCaptureFileSize == 0 {
					maxCaptureFileSize = defaultMaxCaptureSize
				}
				if !resConf.Disabled && (resConf.CaptureFrequencyHz > 0 || svc.maxCaptureFileSize != maxCaptureFileSize) {
					// We only use service-level tags.
					resConf.Tags = svcConfig.Tags

					maxFileSizeChanged := svc.maxCaptureFileSize != maxCaptureFileSize
					svc.maxCaptureFileSize = maxCaptureFileSize

					newCollectorAndConfig, err := svc.initializeOrUpdateCollector(res, componentMethodMetadata, resConf, maxFileSizeChanged)
					if err != nil {
						svc.logger.CErrorw(ctx, "failed to initialize or update collector", "error", err)
					} else {
						newCollectors[componentMethodMetadata] = newCollectorAndConfig
					}
				}
			}
		}
	} else {
		svc.fileDeletionRoutineCancelFn = nil
		svc.fileDeletionBackgroundWorkers = nil
	}

	// If a component/method has been removed from the config, close the collector.
	for md, collAndConfig := range svc.collectors {
		if _, present := newCollectors[md]; !present {
			collAndConfig.Collector.Close()
		}
	}
	svc.collectors = newCollectors
	svc.additionalSyncPaths = svcConfig.AdditionalSyncPaths

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
	if !svc.captureDisabled {
		fileDeletionCtx, cancelFunc := context.WithCancel(context.Background())
		svc.fileDeletionRoutineCancelFn = cancelFunc
		svc.fileDeletionBackgroundWorkers = &sync.WaitGroup{}
		svc.fileDeletionBackgroundWorkers.Add(1)
		go pollFilesystem(fileDeletionCtx, svc.fileDeletionBackgroundWorkers,
			svc.captureDir, deleteEveryNthValue, svc.syncer, svc.logger)
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
		svc.backgroundWorkers.Wait()
		svc.syncRoutineCancelFn = nil
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
						svc.sync()
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

func (svc *builtIn) sync() {
	svc.flushCollectors()

	svc.lock.Lock()
	if svc.syncer != nil {
		toSync := getAllFilesToSync(svc.captureDir, svc.fileLastModifiedMillis)
		for _, ap := range svc.additionalSyncPaths {
			toSync = append(toSync, getAllFilesToSync(ap, svc.fileLastModifiedMillis)...)
		}
		svc.lock.Unlock()

		stopAfter := time.Now().Add(time.Duration(svc.syncIntervalMins * float64(time.Minute)))
		for _, p := range toSync {
			svc.syncer.SyncFile(p, stopAfter)
		}
	} else {
		svc.lock.Unlock()
	}
}

//nolint:errcheck,nilerr
func getAllFilesToSync(dir string, lastModifiedMillis int) []string {
	var filePaths []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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
			filePaths = append(filePaths, path)
		}
		return nil
	})
	return filePaths
}

// Build the component configs associated with the data manager service.
func (svc *builtIn) updateDataCaptureConfigs(
	resources resource.Dependencies,
	conf resource.Config,
	captureDir string,
) (map[resource.Resource][]datamanager.DataCaptureConfig, error) {
	resourceCaptureConfigMap := make(map[resource.Resource][]datamanager.DataCaptureConfig)
	for name, assocCfg := range conf.AssociatedAttributes {
		associatedConf, err := utils.AssertType[*datamanager.AssociatedConfig](assocCfg)
		if err != nil {
			return nil, err
		}

		res, err := resources.Lookup(name)
		if err != nil {
			svc.logger.Debugw("failed to lookup resource", "error", err)
			continue
		}

		captureCopies := make([]datamanager.DataCaptureConfig, len(associatedConf.CaptureMethods))
		for _, method := range associatedConf.CaptureMethods {
			method.CaptureDirectory = captureDir
			captureCopies = append(captureCopies, method)
		}
		resourceCaptureConfigMap[res] = captureCopies
	}
	return resourceCaptureConfigMap, nil
}

func generateMetadataKey(component, method string) string {
	return fmt.Sprintf("%s/%s", component, method)
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
