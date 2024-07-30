// Package capture implements datacapture for the builtin datamanger
package capture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
	"go.viam.com/rdk/services/datamanager/datacapture"
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

// Default time between checking and logging number of files in capture dir.
var captureDirSizeLogInterval = time.Minute

func generateMetadataKey(component, method string) string {
	return fmt.Sprintf("%s/%s", component, method)
}

var metadataToAdditionalParamFields = map[string]string{
	generateMetadataKey("rdk:component:board", "Analogs"): "reader_name",
	generateMetadataKey("rdk:component:board", "Gpios"):   "pin_name",
}

// Capture polls data sources (resource/method pairs) and writes the responses files.
// There must be only one Capture per DataManager.
//
// The lifecycle of a Capture is:
//
// - NewCapture
// - Reconfigure (any number of times)
// - Close (any number of times).
type Capture struct {
	logger          logging.Logger
	clk             clock.Clock
	fileCountLogger *fileCountLogger

	collectorsMu sync.Mutex
	collectors   map[resourceMethodMetadata]*collectorAndConfig

	// captureDir is only stored on Capture so that we can detect when it changs
	captureDir string
	// maxCaptureFileSize is only stored on Capture so that we can detect when it changs
	maxCaptureFileSize int64
	// resourceMethodFrequencyHz is only used to ensure we only log capture frequency values when they change in the config
	resourceMethodFrequencyHz map[resourceMethodMetadata]float32
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

func componentMethodMetadata(resConf datamanager.DataCaptureConfig) resourceMethodMetadata {
	return resourceMethodMetadata{
		ResourceName: resConf.Name.ShortName(),
		MethodMetadata: data.MethodMetadata{
			API:        resConf.Name.API,
			MethodName: resConf.Method,
		},
		MethodParams: fmt.Sprintf("%v", resConf.AdditionalParams),
	}
}

// New creates a new capture manager.
func New(logger logging.Logger, clk clock.Clock) *Capture {
	return &Capture{
		logger:                    logger,
		collectors:                make(map[resourceMethodMetadata]*collectorAndConfig),
		resourceMethodFrequencyHz: make(map[resourceMethodMetadata]float32),
		clk:                       clk,
		fileCountLogger:           newFileCountLogger(logger),
	}
}

// Config is the capture config.
type Config struct {
	CaptureDisabled             bool
	CaptureDir                  string
	Tags                        []string
	MaximumCaptureFileSizeBytes int64
}

func (c *Capture) newCollectors(
	captureMethodConfigsByResource captureMethodConfigsByResource,
	captureConfig Config,
) map[resourceMethodMetadata]*collectorAndConfig {
	// Initialize or add collectors based on changes to the component configurations.
	newCollectors := make(map[resourceMethodMetadata]*collectorAndConfig)
	for resource, captureMethodConfigs := range captureMethodConfigsByResource {
		for _, captureMethodConfig := range captureMethodConfigs {
			md := componentMethodMetadata(captureMethodConfig)
			// logging
			c.maybeLogMethodConfigChange(md, captureMethodConfig)
			// record to minimize duplicate logging
			c.resourceMethodFrequencyHz[md] = captureMethodConfig.CaptureFrequencyHz

			// We only use service-level tags.
			captureMethodConfig.Tags = captureConfig.Tags
			if captureMethodConfig.Disabled {
				c.logger.Debugf("%s disabled. config: %#v", md.String(), captureMethodConfig)
				continue
			}

			if captureMethodConfig.CaptureFrequencyHz <= 0 {
				msg := "%s disabled due to `capture_frequency_hz` being less than or equal to zero. config: %#v"
				c.logger.Debugf(msg, md.String(), captureMethodConfig)
				continue
			}

			newCollectorAndConfig, err := c.initializeOrUpdateCollector(
				resource,
				md,
				captureMethodConfig,
				captureConfig,
			)
			if err != nil {
				c.logger.Errorw("failed to initialize or update collector", "error", err)
				continue
			}
			c.logger.Debugf("%s initialized or updated with config: %#v", md.String(), captureMethodConfig)
			newCollectors[md] = newCollectorAndConfig
		}
	}
	return newCollectors
}

// Reconfigure reconfigures Capture.
// It is only called by the builtin data manager.
func (c *Capture) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	config resource.Config,
	captureConfig Config,
) error {
	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if captureConfig.CaptureDisabled {
		c.logger.Debug("Capture Disabled, flushing & shutting down collectors")
		c.Close()
		return nil
	}

	captureConfig.MaximumCaptureFileSizeBytes = defaultIfZeroVal(captureConfig.MaximumCaptureFileSizeBytes, defaultMaxCaptureSize)
	if c.captureDir != captureConfig.CaptureDir {
		c.fileCountLogger.reconfigure(captureConfig.CaptureDir)
	}

	captureMethodConfigsByResource, err := getCaptureMethodConfigsByResource(deps, config, captureConfig.CaptureDir, c.logger)
	if err != nil {
		// This would only happen if there is a bug in resource graph
		return err
	}
	newCollectors := c.newCollectors(
		captureMethodConfigsByResource,
		captureConfig,
	)

	// If a component/method has been removed from the config, close the collector.
	for md, collAndConfig := range c.collectors {
		if _, present := newCollectors[md]; !present {
			c.logger.Debugf("%s closing collector", md.String())
			collAndConfig.Collector.Close()
		}
	}
	c.collectors = newCollectors
	c.captureDir = captureConfig.CaptureDir
	c.maxCaptureFileSize = captureConfig.MaximumCaptureFileSizeBytes
	return nil
}

// Close closes the capture manager.
func (c *Capture) Close() {
	c.FlushCollectors()
	c.CloseCollectors()
	c.fileCountLogger.close()
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (c *Capture) initializeOrUpdateCollector(
	res resource.Resource,
	md resourceMethodMetadata,
	resourceMethodConfig datamanager.DataCaptureConfig,
	config Config,
) (*collectorAndConfig, error) {
	// TODO(DATA-451): validate method params
	methodParams, err := protoutils.ConvertStringMapToAnyPBMap(resourceMethodConfig.AdditionalParams)
	if err != nil {
		return nil, err
	}

	maxFileSizeChanged := c.maxCaptureFileSize != config.MaximumCaptureFileSizeBytes
	if storedCollectorAndConfig, ok := c.collectors[md]; ok {
		if storedCollectorAndConfig.Config.Equals(&resourceMethodConfig) &&
			res == storedCollectorAndConfig.Resource &&
			!maxFileSizeChanged {
			// If the attributes have not changed, do nothing and leave the existing collector.
			return c.collectors[md], nil
		}
		// If the attributes have changed, close the existing collector.
		storedCollectorAndConfig.Collector.Close()
	}

	// Get collector constructor for the component API and method.
	collectorConstructor := data.CollectorLookup(md.MethodMetadata)
	if collectorConstructor == nil {
		return nil, errors.Errorf("failed to find collector constructor for %s", md.MethodMetadata)
	}

	metadataKey := generateMetadataKey(md.MethodMetadata.API.String(), md.MethodMetadata.MethodName)
	additionalParamKey, ok := metadataToAdditionalParamFields[metadataKey]
	if ok {
		if _, ok := resourceMethodConfig.AdditionalParams[additionalParamKey]; !ok {
			return nil, errors.Errorf("failed to validate additional_params for %s, must supply %s",
				md.MethodMetadata.API.String(), additionalParamKey)
		}
	}

	// Create a collector for this resource and method.
	targetDir := datacapture.FilePathWithReplacedReservedChars(
		filepath.Join(config.CaptureDir, resourceMethodConfig.Name.API.String(),
			resourceMethodConfig.Name.ShortName(), resourceMethodConfig.Method))
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, err
	}
	// Build metadata.
	captureMetadata := datacapture.BuildCaptureMetadata(
		resourceMethodConfig.Name.API,
		resourceMethodConfig.Name.ShortName(),
		resourceMethodConfig.Method,
		resourceMethodConfig.AdditionalParams,
		methodParams,
		resourceMethodConfig.Tags,
	)
	// Parameters to initialize collector.
	collector, err := collectorConstructor(res, data.CollectorParams{
		ComponentName: resourceMethodConfig.Name.ShortName(),
		Interval:      data.GetDurationFromHz(resourceMethodConfig.CaptureFrequencyHz),
		MethodParams:  methodParams,
		Target:        datacapture.NewBuffer(targetDir, captureMetadata, config.MaximumCaptureFileSizeBytes),
		// Set queue size to defaultCaptureQueueSize if it was not set in the config.
		QueueSize:  defaultIfZeroVal(resourceMethodConfig.CaptureQueueSize, defaultCaptureQueueSize),
		BufferSize: defaultIfZeroVal(resourceMethodConfig.CaptureBufferSize, defaultCaptureBufferSize),
		Logger:     c.logger,
		Clock:      c.clk,
	})
	if err != nil {
		return nil, err
	}

	collector.Collect()

	return &collectorAndConfig{res, collector, resourceMethodConfig}, nil
}

type captureMethodConfigsByResource map[resource.Resource][]datamanager.DataCaptureConfig

// Build the component configs associated with the data manager service.
func getCaptureMethodConfigsByResource(
	resources resource.Dependencies,
	conf resource.Config,
	captureDir string,
	logger logging.Logger,
) (captureMethodConfigsByResource, error) {
	captureMethodConfigsByResource := captureMethodConfigsByResource{}
	for name, assocCfg := range conf.AssociatedAttributes {
		associatedConf, err := utils.AssertType[*datamanager.AssociatedConfig](assocCfg)
		if err != nil {
			// This would only happen if there is a bug in resource graph
			return nil, err
		}
		res, err := resources.Lookup(name)
		if err != nil {
			logger.Warnw("datamanager failed to lookup resource from config", "error", err)
			continue
		}

		captureMethodConfigs := []datamanager.DataCaptureConfig{}
		for _, captureMethodConfig := range associatedConf.CaptureMethods {
			// we need to set the CaptureDirectory to that in the data manager config
			captureMethodConfig.CaptureDirectory = captureDir
			captureMethodConfigs = append(captureMethodConfigs, captureMethodConfig)
		}
		captureMethodConfigsByResource[res] = captureMethodConfigs
	}
	return captureMethodConfigsByResource, nil
}

// CloseCollectors closes collectors.
func (c *Capture) CloseCollectors() {
	var collectorsToClose []data.Collector
	c.collectorsMu.Lock()
	for _, collectorAndConfig := range c.collectors {
		collectorsToClose = append(collectorsToClose, collectorAndConfig.Collector)
	}
	c.collectors = make(map[resourceMethodMetadata]*collectorAndConfig)
	c.collectorsMu.Unlock()

	var wg sync.WaitGroup
	for _, collector := range collectorsToClose {
		tmp := collector
		wg.Add(1)
		goutils.ManagedGo(tmp.Close, wg.Done)
	}
	wg.Wait()
}

// FlushCollectors flushes collectors.
func (c *Capture) FlushCollectors() {
	var collectorsToFlush []data.Collector
	c.collectorsMu.Lock()
	for _, collectorAndConfig := range c.collectors {
		collectorsToFlush = append(collectorsToFlush, collectorAndConfig.Collector)
	}
	c.collectorsMu.Unlock()

	var wg sync.WaitGroup
	for _, collector := range collectorsToFlush {
		tmp := collector
		wg.Add(1)
		goutils.ManagedGo(tmp.Flush, wg.Done)
	}
	wg.Wait()
}

func (c *Capture) maybeLogMethodConfigChange(
	componentMethodMetadata resourceMethodMetadata,
	resConf datamanager.DataCaptureConfig,
) {
	prevFreqHz, ok := c.resourceMethodFrequencyHz[componentMethodMetadata]

	// Only log capture frequency if the component frequency is new or the frequency has changed
	// otherwise we'll be logging way too much
	// equal to a bit more than one iteration a day
	newCaptureFreq := !ok || !utils.Float32AlmostEqual(resConf.CaptureFrequencyHz, prevFreqHz, 0.00001)
	if newCaptureFreq {
		syncVal := "will"
		if resConf.CaptureFrequencyHz == 0 || resConf.Disabled {
			syncVal += " not"
		}
		c.logger.Infof(
			"capture frequency for %s is set to %.2fHz and %s sync", componentMethodMetadata, resConf.CaptureFrequencyHz, syncVal,
		)
	}
}

func countFiles(ctx context.Context, captureDir string) int {
	numFiles := 0
	goutils.UncheckedError(filepath.Walk(captureDir, func(path string, info os.FileInfo, err error) error {
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
	}))
	return numFiles
}

func defaultIfZeroVal[T comparable](val, defaultVal T) T {
	var zeroVal T
	if val == zeroVal {
		return defaultVal
	}
	return val
}

type fileCountLogger struct {
	logger logging.Logger

	// mu      sync.Mutex
	workers utils.StoppableWorkers
}

func newFileCountLogger(logger logging.Logger) *fileCountLogger {
	return &fileCountLogger{
		logger:  logger,
		workers: utils.NewStoppableWorkers(),
	}
}

func (poller *fileCountLogger) reconfigure(captureDir string) {
	// poller.mu.Lock()
	// defer poller.mu.Unlock()

	poller.workers.Stop()
	poller.workers = utils.NewStoppableWorkers(func(stopCtx context.Context) {
		t := time.NewTicker(captureDirSizeLogInterval)
		defer t.Stop()
		for {
			select {
			case <-stopCtx.Done():
				return
			case <-t.C:
				numFiles := countFiles(stopCtx, captureDir)
				if numFiles > minNumFiles {
					poller.logger.Infof("Capture dir contains %d files", numFiles)
				}
			}
		}
	})
}

func (poller *fileCountLogger) close() {
	// poller.mu.Lock()
	// defer poller.mu.Unlock()
	poller.workers.Stop()
}
