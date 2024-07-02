package data

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/utils"
)

// TODO: re-determine if queue size is optimal given we now support 10khz+ capture rates
// The Collector's queue should be big enough to ensure that .capture() is never blocked by the queue being
// written to disk. A default value of 250 was chosen because even with the fastest reasonable capture interval (1ms),
// this would leave 250ms for a (buffered) disk write before blocking, which seems sufficient for the size of
// writes this would be performing.
const defaultCaptureQueueSize = 250

// Default bufio.Writer buffer size in bytes.
const defaultCaptureBufferSize = 4096

// Threshold number of files to check if sync is backed up (defined as >1000 files).
var minNumFiles = 1000

// Default maximum size in bytes of a data capture file.
var defaultMaxCaptureSize = int64(256 * 1024)

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), ".viam", "capture")

// Default time between checking and logging number of files in capture dir.
var captureDirSizeLogInterval = 1 * time.Minute

// ErrCaptureDirectoryConfigurationDisabled happens when the viam-server is run with
// `-untrusted-env` and the capture directory is not `~/.viam`.
var ErrCaptureDirectoryConfigurationDisabled = errors.New("changing the capture directory is prohibited in this environment")

func generateMetadataKey(component, method string) string {
	return fmt.Sprintf("%s/%s", component, method)
}

var metadataToAdditionalParamFields = map[string]string{
	generateMetadataKey("rdk:component:board", "Analogs"): "reader_name",
	generateMetadataKey("rdk:component:board", "Gpios"):   "pin_name",
}

// CaptureManager manages polling resources for metrics and writing those metrics to files. There
// must be only one CaptureManager per DataManager. The lifecycle of a CaptureManager is:
//
// - NewCaptureManager
// - Reconfigure (any number of times)
// - Close (once).
type CaptureManager struct {
	mu                         sync.Mutex
	captureDir                 string
	captureDisabled            bool
	collectors                 map[resourceMethodMetadata]*collectorAndConfig
	maxCaptureFileSize         int64
	componentMethodFrequencyHz map[resourceMethodMetadata]float32

	captureDirPollingCancelFn          context.CancelFunc
	captureDirPollingBackgroundWorkers *sync.WaitGroup

	logger logging.Logger
	clk    clock.Clock
}

// NewCaptureManager creates a new capture manager.
func NewCaptureManager(logger logging.Logger, clk clock.Clock) *CaptureManager {
	return &CaptureManager{
		logger:                     logger,
		captureDir:                 viamCaptureDotDir,
		collectors:                 make(map[resourceMethodMetadata]*collectorAndConfig),
		componentMethodFrequencyHz: make(map[resourceMethodMetadata]float32),
		clk:                        clk,
	}
}

// Config is the capture manager config.
type Config struct {
	CaptureDisabled             bool
	CaptureDir                  string
	Tags                        []string
	MaximumCaptureFileSizeBytes int64
}

// Reconfigure reconfigures the capture manager.
func (cm *CaptureManager) Reconfigure(ctx context.Context, deps resource.Dependencies, resConfig resource.Config, dataConfig Config) error {
	captureConfigs, err := cm.updateDataCaptureConfigs(deps, resConfig, dataConfig.CaptureDir)
	if err != nil {
		return err
	}

	if !utils.IsTrustedEnvironment(ctx) && dataConfig.CaptureDir != "" && dataConfig.CaptureDir != viamCaptureDotDir {
		return ErrCaptureDirectoryConfigurationDisabled
	}

	if dataConfig.CaptureDir != "" {
		cm.captureDir = dataConfig.CaptureDir
	} else {
		cm.captureDir = viamCaptureDotDir
	}
	cm.captureDisabled = dataConfig.CaptureDisabled
	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if cm.captureDisabled {
		cm.CloseCollectors()
		cm.collectors = make(map[resourceMethodMetadata]*collectorAndConfig)
	}

	// Initialize or add collectors based on changes to the component configurations.
	newCollectors := make(map[resourceMethodMetadata]*collectorAndConfig)
	if !cm.captureDisabled {
		for res, resConfs := range captureConfigs {
			for _, resConf := range resConfs {
				if resConf.Method == "" {
					continue
				}
				// Create component/method metadata
				methodMetadata := MethodMetadata{
					API:        resConf.Name.API,
					MethodName: resConf.Method,
				}

				componentMethodMetadata := resourceMethodMetadata{
					ResourceName:   resConf.Name.ShortName(),
					MethodMetadata: methodMetadata,
					MethodParams:   fmt.Sprintf("%v", resConf.AdditionalParams),
				}
				_, ok := cm.componentMethodFrequencyHz[componentMethodMetadata]

				// Only log capture frequency if the component frequency is new or the frequency has changed
				// otherwise we'll be logging way too much
				if !ok || (ok && resConf.CaptureFrequencyHz != cm.componentMethodFrequencyHz[componentMethodMetadata]) {
					syncVal := "will"
					if resConf.CaptureFrequencyHz == 0 {
						syncVal += " not"
					}
					cm.logger.Infof(
						"capture frequency for %s is set to %.2fHz and %s sync", componentMethodMetadata, resConf.CaptureFrequencyHz, syncVal,
					)
				}

				// we need this map to keep track of if state has changed in the configs
				// without it, we will be logging the same message over and over for no reason
				cm.componentMethodFrequencyHz[componentMethodMetadata] = resConf.CaptureFrequencyHz

				maxCaptureFileSize := dataConfig.MaximumCaptureFileSizeBytes
				if maxCaptureFileSize == 0 {
					maxCaptureFileSize = defaultMaxCaptureSize
				}
				if !resConf.Disabled && (resConf.CaptureFrequencyHz > 0 || cm.maxCaptureFileSize != maxCaptureFileSize) {
					// We only use service-level tags.
					resConf.Tags = dataConfig.Tags

					maxFileSizeChanged := cm.maxCaptureFileSize != maxCaptureFileSize
					cm.maxCaptureFileSize = maxCaptureFileSize

					newCollectorAndConfig, err := cm.initializeOrUpdateCollector(res, componentMethodMetadata, resConf, maxFileSizeChanged)
					if err != nil {
						cm.logger.CErrorw(ctx, "failed to initialize or update collector", "error", err)
					} else {
						newCollectors[componentMethodMetadata] = newCollectorAndConfig
					}
				}
			}
		}
	}

	// If a component/method has been removed from the config, close the collector.
	for md, collAndConfig := range cm.collectors {
		if _, present := newCollectors[md]; !present {
			collAndConfig.Collector.Close()
		}
	}
	cm.collectors = newCollectors

	if cm.captureDirPollingCancelFn != nil {
		cm.captureDirPollingCancelFn()
	}
	if cm.captureDirPollingBackgroundWorkers != nil {
		cm.captureDirPollingBackgroundWorkers.Wait()
	}
	captureDirPollCtx, captureDirCancelFunc := context.WithCancel(context.Background())
	cm.captureDirPollingCancelFn = captureDirCancelFunc
	cm.captureDirPollingBackgroundWorkers = &sync.WaitGroup{}
	cm.captureDirPollingBackgroundWorkers.Add(1)
	go cm.logCaptureDirSize(captureDirPollCtx, cm.captureDir, cm.captureDirPollingBackgroundWorkers, cm.logger)

	return nil
}

// Close closes the capture manager.
func (cm *CaptureManager) Close() {
	if cm.captureDirPollingCancelFn != nil {
		cm.captureDirPollingCancelFn()
	}
	if cm.captureDirPollingBackgroundWorkers != nil {
		cm.captureDirPollingBackgroundWorkers.Wait()
	}

	cm.FlushCollectors()
	cm.CloseCollectors()
}

// CaptureDir returns the capture directory.
func (cm *CaptureManager) CaptureDir() string {
	return cm.captureDir
}

// Parameters stored for each collector.
type collectorAndConfig struct {
	Resource  resource.Resource
	Collector Collector
	Config    datamanager.DataCaptureConfig
}

// Identifier for a particular collector: component name, component model, component type,
// method parameters, and method name.
type resourceMethodMetadata struct {
	ResourceName   string
	MethodParams   string
	MethodMetadata MethodMetadata
}

func (r resourceMethodMetadata) String() string {
	return fmt.Sprintf(
		"[API: %s, Resource Name: %s, Method Name: %s, Method Params: %s]",
		r.MethodMetadata.API, r.ResourceName, r.MethodMetadata.MethodName, r.MethodParams)
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (cm *CaptureManager) initializeOrUpdateCollector(
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

	if storedCollectorAndConfig, ok := cm.collectors[md]; ok {
		if storedCollectorAndConfig.Config.Equals(&config) &&
			res == storedCollectorAndConfig.Resource &&
			!maxFileSizeChanged {
			// If the attributes have not changed, do nothing and leave the existing collector.
			return cm.collectors[md], nil
		}
		// If the attributes have changed, close the existing collector.
		storedCollectorAndConfig.Collector.Close()
	}

	// Get collector constructor for the component API and method.
	collectorConstructor := CollectorLookup(md.MethodMetadata)
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
		filepath.Join(cm.captureDir, captureMetadata.GetComponentType(),
			captureMetadata.GetComponentName(), captureMetadata.GetMethodName()))
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, err
	}
	params := CollectorParams{
		ComponentName: config.Name.ShortName(),
		Interval:      interval,
		MethodParams:  methodParams,
		Target:        datacapture.NewBuffer(targetDir, captureMetadata, cm.maxCaptureFileSize),
		QueueSize:     captureQueueSize,
		BufferSize:    captureBufferSize,
		Logger:        cm.logger,
		Clock:         cm.clk,
	}
	collector, err := collectorConstructor(res, params)
	if err != nil {
		return nil, err
	}
	collector.Collect()

	return &collectorAndConfig{res, collector, config}, nil
}

// Build the component configs associated with the data manager service.
func (cm *CaptureManager) updateDataCaptureConfigs(
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
			cm.logger.Debugw("failed to lookup resource", "error", err)
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

// CloseCollectors closes collectors.
func (cm *CaptureManager) CloseCollectors() {
	var collectorsToClose []Collector
	cm.mu.Lock()
	for _, collectorAndConfig := range cm.collectors {
		collectorsToClose = append(collectorsToClose, collectorAndConfig.Collector)
	}
	cm.collectors = make(map[resourceMethodMetadata]*collectorAndConfig)
	cm.mu.Unlock()

	var wg sync.WaitGroup
	for _, collector := range collectorsToClose {
		wg.Add(1)
		go func(toClose Collector) {
			defer wg.Done()
			toClose.Close()
		}(collector)
	}
	wg.Wait()
}

// FlushCollectors flushes collectors.
func (cm *CaptureManager) FlushCollectors() {
	var collectorsToFlush []Collector
	cm.mu.Lock()
	for _, collectorAndConfig := range cm.collectors {
		collectorsToFlush = append(collectorsToFlush, collectorAndConfig.Collector)
	}
	cm.mu.Unlock()

	var wg sync.WaitGroup
	for _, collector := range collectorsToFlush {
		wg.Add(1)
		go func(toFlush Collector) {
			defer wg.Done()
			toFlush.Flush()
		}(collector)
	}
	wg.Wait()
}

// Get time.Duration from hz.
func getDurationFromHz(captureFrequencyHz float32) time.Duration {
	if captureFrequencyHz == 0 {
		return time.Duration(0)
	}
	return time.Duration(float32(time.Second) / captureFrequencyHz)
}

func (cm *CaptureManager) logCaptureDirSize(ctx context.Context, captureDir string, wg *sync.WaitGroup, logger logging.Logger) {
	t := cm.clk.Ticker(captureDirSizeLogInterval)
	defer t.Stop()
	defer wg.Done()
	for {
		if err := ctx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Errorw("data manager context closed unexpectedly", "error", err)
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			numFiles := countCaptureDirFiles(ctx, captureDir)
			if numFiles > minNumFiles {
				logger.Infof("Capture dir contains %d files", numFiles)
			}
		}
	}
}

func countCaptureDirFiles(ctx context.Context, captureDir string) int {
	numFiles := 0
	//nolint:errcheck
	_ = filepath.Walk(captureDir, func(path string, info os.FileInfo, err error) error {
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		//nolint:nilerr
		if err != nil {
			return nil
		}

		// Do not count the files in the corrupted data directory.
		if info.IsDir() && info.Name() == datasync.FailedDir {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}
		// this is intentionally not doing as many checkas as getAllFilesToSync because
		// this is intended for debugging and does not need to be 100% accurate.
		isCompletedCaptureFile := filepath.Ext(path) == datacapture.FileExt
		if isCompletedCaptureFile {
			numFiles++
		}
		return nil
	})
	return numFiles
}
