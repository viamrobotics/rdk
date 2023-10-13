// Package builtin contains a service type that can be used to capture data from a robot's components.
package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/utils"
)

func init() {
	resource.RegisterDefaultService(
		datamanager.API,
		resource.DefaultServiceModel,
		resource.Registration[datamanager.Service, *Config]{
			Constructor: NewBuiltIn,
			AssociatedConfigLinker: func(conf *Config, resAssociation interface{}) error {
				capConf, err := utils.AssertType[*datamanager.DataCaptureConfigs](resAssociation)
				if err != nil {
					return err
				}
				for _, method := range capConf.CaptureMethods {
					methodCopy := method
					conf.ResourceConfigs = append(conf.ResourceConfigs, &methodCopy)
				}

				return nil
			},
			// NOTE(erd): this would be better as a weak dependencies returned through a more
			// typed validate or different system.
			WeakDependencies: []internal.ResourceMatcher{internal.ComponentDependencyWildcardMatcher, internal.SLAMDependencyWildcardMatcher},
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

var clock = clk.New()

var errCaptureDirectoryConfigurationDisabled = errors.New("changing the capture directory is prohibited in this environment")

// Config describes how to configure the service.
type Config struct {
	CaptureDir            string                           `json:"capture_dir"`
	AdditionalSyncPaths   []string                         `json:"additional_sync_paths"`
	SyncIntervalMins      float64                          `json:"sync_interval_mins"`
	CaptureDisabled       bool                             `json:"capture_disabled"`
	ScheduledSyncDisabled bool                             `json:"sync_disabled"`
	Tags                  []string                         `json:"tags"`
	ResourceConfigs       []*datamanager.DataCaptureConfig `json:"resource_configs"`
}

// Validate returns components which will be depended upon weakly due to the above matcher.
func (c *Config) Validate(path string) ([]string, error) {
	return []string{cloud.InternalServiceName.String()}, nil
}

// builtIn initializes and orchestrates data capture collectors for registered component/methods.
type builtIn struct {
	resource.Named
	logger                      golog.Logger
	captureDir                  string
	captureDisabled             bool
	collectors                  map[resourceMethodMetadata]*collectorAndConfig
	lock                        sync.Mutex
	backgroundWorkers           sync.WaitGroup
	waitAfterLastModifiedMillis int

	additionalSyncPaths []string
	tags                []string
	syncDisabled        bool
	syncIntervalMins    float64
	syncRoutineCancelFn context.CancelFunc
	syncer              datasync.Manager
	syncerConstructor   datasync.ManagerConstructor
	cloudConnSvc        cloud.ConnectionService
	cloudConn           rpc.ClientConn
	syncTicker          *clk.Ticker

	corruptedFilesDir string
}

var (
	viamCaptureDotDir   = filepath.Join(os.Getenv("HOME"), ".viam", "capture")
	viamCorruptedDotDir = filepath.Join(os.Getenv("HOME"), ".viam", "corrupted_capture")
)

// NewBuiltIn returns a new data manager service for the given robot.
func NewBuiltIn(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (datamanager.Service, error) {
	svc := &builtIn{
		Named:                       conf.ResourceName().AsNamed(),
		logger:                      logger,
		captureDir:                  viamCaptureDotDir,
		collectors:                  make(map[resourceMethodMetadata]*collectorAndConfig),
		syncIntervalMins:            0,
		additionalSyncPaths:         []string{},
		tags:                        []string{},
		waitAfterLastModifiedMillis: 10000,
		syncerConstructor:           datasync.NewManager,
		corruptedFilesDir:           viamCorruptedDotDir,
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
	svc.cancelSyncScheduler()

	svc.lock.Unlock()
	svc.backgroundWorkers.Wait()
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

// Get time.Duration from hz.
func getDurationFromHz(captureFrequencyHz float32) time.Duration {
	if captureFrequencyHz == 0 {
		return time.Duration(0)
	}
	return time.Duration(float32(time.Second) / captureFrequencyHz)
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (svc *builtIn) initializeOrUpdateCollector(
	md resourceMethodMetadata,
	config *datamanager.DataCaptureConfig,
) (
	*collectorAndConfig, error,
) {
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
		if storedCollectorAndConfig.Config.Equals(config) {
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

	methodParams, err := protoutils.ConvertStringMapToAnyPBMap(config.AdditionalParams)
	if err != nil {
		return nil, err
	}

	// Create a collector for this resource and method.
	targetDir := filepath.Join(svc.captureDir, captureMetadata.GetComponentType(), captureMetadata.GetComponentName(),
		captureMetadata.GetMethodName())
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, err
	}
	params := data.CollectorParams{
		ComponentName: config.Name.ShortName(),
		Interval:      interval,
		MethodParams:  methodParams,
		Target:        datacapture.NewBuffer(targetDir, captureMetadata),
		QueueSize:     captureQueueSize,
		BufferSize:    captureBufferSize,
		Logger:        svc.logger,
		Clock:         clock,
	}
	collector, err := (*collectorConstructor)(config.Resource, params)
	if err != nil {
		return nil, err
	}
	collector.Collect()

	return &collectorAndConfig{collector, *config}, nil
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
		svc.logger.Debug("Using no-op sync manager when not cloud managed")
		svc.syncer = datasync.NewNoopManager()
	}
	if err != nil {
		return err
	}

	client := v1.NewDataSyncServiceClient(conn)

	syncer, err := svc.syncerConstructor(identity, client, svc.logger, svc.corruptedFilesDir)
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
func (svc *builtIn) Sync(ctx context.Context, _ map[string]interface{}) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.syncer == nil {
		err := svc.initSyncer(ctx)
		if err != nil {
			return err
		}
	}

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

	reinitSyncer := cloudConnSvc != svc.cloudConnSvc
	svc.cloudConnSvc = cloudConnSvc

	svc.updateDataCaptureConfigs(deps, svcConfig.ResourceConfigs, svcConfig.CaptureDir)

	if !utils.IsTrustedEnvironment(ctx) && svcConfig.CaptureDir != "" && svcConfig.CaptureDir != viamCaptureDotDir {
		return errCaptureDirectoryConfigurationDisabled
	}

	if svcConfig.CaptureDir != "" {
		svc.captureDir = svcConfig.CaptureDir
	}
	if svcConfig.CaptureDir == "" {
		svc.captureDir = viamCaptureDotDir
	}
	svc.captureDisabled = svcConfig.CaptureDisabled
	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if svc.captureDisabled {
		svc.closeCollectors()
		svc.collectors = make(map[resourceMethodMetadata]*collectorAndConfig)
	}

	// Initialize or add collectors based on changes to the component configurations.
	newCollectors := make(map[resourceMethodMetadata]*collectorAndConfig)
	if !svc.captureDisabled {
		for _, resConf := range svcConfig.ResourceConfigs {
			if resConf.Resource == nil {
				// do not have the resource right now
				continue
			}
			if !resConf.Disabled && resConf.CaptureFrequencyHz > 0 {
				// Create component/method metadata to check if the collector exists.
				methodMetadata := data.MethodMetadata{
					API:        resConf.Name.API,
					MethodName: resConf.Method,
				}

				componentMethodMetadata := resourceMethodMetadata{
					ResourceName:   resConf.Name.ShortName(),
					MethodMetadata: methodMetadata,
					MethodParams:   fmt.Sprintf("%v", resConf.AdditionalParams),
				}

				// We only use service-level tags.
				resConf.Tags = svcConfig.Tags

				newCollectorAndConfig, err := svc.initializeOrUpdateCollector(componentMethodMetadata, resConf)
				if err != nil {
					svc.logger.Errorw("failed to initialize or update collector", "error", err)
				} else {
					newCollectors[componentMethodMetadata] = newCollectorAndConfig
				}
			}
		}
	}

	// If a component/method has been removed from the config, close the collector.
	for md, collAndConfig := range svc.collectors {
		if _, present := newCollectors[md]; !present {
			collAndConfig.Collector.Close()
		}
	}
	svc.collectors = newCollectors
	svc.additionalSyncPaths = svcConfig.AdditionalSyncPaths

	if svc.syncDisabled != svcConfig.ScheduledSyncDisabled || svc.syncIntervalMins != svcConfig.SyncIntervalMins ||
		!reflect.DeepEqual(svc.tags, svcConfig.Tags) {
		svc.syncDisabled = svcConfig.ScheduledSyncDisabled
		svc.syncIntervalMins = svcConfig.SyncIntervalMins
		svc.tags = svcConfig.Tags

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
					svc.sync()
				}
				svc.lock.Unlock()
			}
		}
	})
}

func (svc *builtIn) sync() {
	svc.flushCollectors()
	toSync := getAllFilesToSync(svc.captureDir, svc.waitAfterLastModifiedMillis)
	for _, ap := range svc.additionalSyncPaths {
		toSync = append(toSync, getAllFilesToSync(ap, svc.waitAfterLastModifiedMillis)...)
	}
	for _, p := range toSync {
		svc.syncer.SyncFile(p)
	}
}

//nolint
func getAllFilesToSync(dir string, lastModifiedMillis int) []string {
	var filePaths []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// If a file was modified within the past lastModifiedMillis seconds, do not sync it (data
		// may still be being written).
		timeSinceMod := clock.Since(info.ModTime())
		// When using a mock clock in tests, this can be negative since the file system will still use the system clock.
		// Take max(timeSinceMod, 0) to account for this.
		if timeSinceMod < 0 {
			timeSinceMod = 0
		}
		if timeSinceMod >= (time.Duration(lastModifiedMillis)*time.Millisecond) || filepath.Ext(path) == datacapture.FileExt {
			filePaths = append(filePaths, path)
		}
		return nil
	})
	return filePaths
}

// Build the component configs associated with the data manager service.
func (svc *builtIn) updateDataCaptureConfigs(
	resources resource.Dependencies,
	resourceConfigs []*datamanager.DataCaptureConfig,
	captureDir string,
) {
	for _, resConf := range resourceConfigs {
		res, err := resources.Lookup(resConf.Name)
		if err != nil {
			svc.logger.Debugw("failed to lookup resource", "error", err)
			continue
		}

		resConf.Resource = res
		resConf.CaptureDirectory = captureDir
	}
}
