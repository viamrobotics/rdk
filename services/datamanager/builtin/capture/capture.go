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
// - Close (once).
type Capture struct {
	logger logging.Logger
	clk    clock.Clock

	collectorsMu sync.Mutex
	collectors   map[resourceMethodMetadata]*collectorAndConfig

	captureDir                 string
	maxCaptureFileSize         int64
	componentMethodFrequencyHz map[resourceMethodMetadata]float32
	capturePolling             *captureDirPoller
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
		logger:                     logger,
		collectors:                 make(map[resourceMethodMetadata]*collectorAndConfig),
		componentMethodFrequencyHz: make(map[resourceMethodMetadata]float32),
		clk:                        clk,
	}
}

// Config is the capture config.
type Config struct {
	CaptureDisabled             bool
	CaptureDir                  string
	Tags                        []string
	MaximumCaptureFileSizeBytes int64
}

// Reconfigure reconfigures Capture.
// It is only called by the builtin data manager.
func (c *Capture) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	config resource.Config,
	captureConfig Config,
) error {
	if c.capturePolling == nil {
		c.capturePolling = newCaptureDirPoller(c.logger)
	}

	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if captureConfig.CaptureDisabled {
		c.logger.Debug("Capture Disabled")
		c.Close()
		return nil
	}
	c.capturePolling.reconfigure(c.captureDir)

	// TODO: How does this happen?
	dataCollectorConfigsByResource, err := c.getDataCollectorConfigs(deps, config, captureConfig.CaptureDir)
	if err != nil {
		// This would only happen if there is a bug in resource graph
		return err
	}
	c.captureDir = captureConfig.CaptureDir
	maxCaptureFileSize := defaultIfZeroVal(captureConfig.MaximumCaptureFileSizeBytes, defaultMaxCaptureSize)
	maxFileSizeChanged := c.maxCaptureFileSize != maxCaptureFileSize
	c.maxCaptureFileSize = maxCaptureFileSize

	// Initialize or add collectors based on changes to the component configurations.
	newCollectors := make(map[resourceMethodMetadata]*collectorAndConfig)
	for res, dataCaptgureConfigs := range dataCollectorConfigsByResource {
		for _, resConf := range dataCaptgureConfigs {
			componentMethodMetadata := componentMethodMetadata(resConf)
			// logging
			c.maybeLogCollectorConfigChange(componentMethodMetadata, resConf)
			// record to minimize duplicate logging
			c.componentMethodFrequencyHz[componentMethodMetadata] = resConf.CaptureFrequencyHz

			// We only use service-level tags.
			resConf.Tags = captureConfig.Tags
			if resConf.Disabled {
				c.logger.Debugf("%s disabled. config: %#v", componentMethodMetadata.String(), resConf)
				continue
			}

			if resConf.CaptureFrequencyHz <= 0 {
				msg := "%s disabled due to `capture_frequency_hz` being less than or equal to zero. config: %#v"
				c.logger.Debugf(msg, componentMethodMetadata.String(), resConf)
				continue
			}

			newCollectorAndConfig, err := c.initializeOrUpdateCollector(res, componentMethodMetadata, resConf, maxFileSizeChanged)
			if err != nil {
				c.logger.CErrorw(ctx, "failed to initialize or update collector", "error", err)
				continue
			}
			c.logger.Debugf("%s initialized or updated with config: %#v", componentMethodMetadata.String(), resConf)
			newCollectors[componentMethodMetadata] = newCollectorAndConfig
		}
	}

	// If a component/method has been removed from the config, close the collector.
	for md, collAndConfig := range c.collectors {
		if _, present := newCollectors[md]; !present {
			c.logger.Debugf("%s closing collector", md.String())
			collAndConfig.Collector.Close()
		}
	}
	c.collectors = newCollectors

	return nil
}

// Close closes the capture manager.
func (c *Capture) Close() {
	c.FlushCollectors()
	c.CloseCollectors()
	c.capturePolling.close()
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (c *Capture) initializeOrUpdateCollector(
	res resource.Resource,
	md resourceMethodMetadata,
	config datamanager.DataCaptureConfig,
	maxFileSizeChanged bool,
) (*collectorAndConfig, error) {
	// TODO(DATA-451): validate method params
	methodParams, err := protoutils.ConvertStringMapToAnyPBMap(config.AdditionalParams)
	if err != nil {
		return nil, err
	}

	if storedCollectorAndConfig, ok := c.collectors[md]; ok {
		if storedCollectorAndConfig.Config.Equals(&config) &&
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

	// Set queue size to defaultCaptureQueueSize if it was not set in the config.
	captureQueueSize := defaultIfZeroVal(config.CaptureQueueSize, defaultCaptureQueueSize)
	captureBufferSize := defaultIfZeroVal(config.CaptureBufferSize, defaultCaptureBufferSize)

	metadataKey := generateMetadataKey(
		md.MethodMetadata.API.String(),
		md.MethodMetadata.MethodName)
	additionalParamKey, ok := metadataToAdditionalParamFields[metadataKey]
	if ok {
		if _, ok := config.AdditionalParams[additionalParamKey]; !ok {
			return nil, errors.Errorf("failed to validate additional_params for %s, must supply %s",
				md.MethodMetadata.API.String(), additionalParamKey)
		}
	}

	// Create a collector for this resource and method.
	targetDir := datacapture.FilePathWithReplacedReservedChars(
		filepath.Join(c.captureDir, config.Name.API.String(),
			config.Name.ShortName(), config.Method))
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, err
	}
	// Build metadata.
	captureMetadata := datacapture.BuildCaptureMetadata(
		config.Name.API,
		config.Name.ShortName(),
		config.Method,
		config.AdditionalParams,
		methodParams,
		config.Tags,
	)
	// Parameters to initialize collector.
	interval := data.GetDurationFromHz(config.CaptureFrequencyHz)
	collector, err := collectorConstructor(res, data.CollectorParams{
		ComponentName: config.Name.ShortName(),
		Interval:      interval,
		MethodParams:  methodParams,
		Target:        datacapture.NewBuffer(targetDir, captureMetadata, c.maxCaptureFileSize),
		QueueSize:     captureQueueSize,
		BufferSize:    captureBufferSize,
		Logger:        c.logger,
		Clock:         c.clk,
	})
	if err != nil {
		return nil, err
	}
	collector.Collect()

	return &collectorAndConfig{res, collector, config}, nil
}

// Build the component configs associated with the data manager service.
func (c *Capture) getDataCollectorConfigs(
	resources resource.Dependencies,
	conf resource.Config,
	captureDir string,
) (map[resource.Resource][]datamanager.DataCaptureConfig, error) {
	resourceCaptureConfigMap := make(map[resource.Resource][]datamanager.DataCaptureConfig)
	for name, assocCfg := range conf.AssociatedAttributes {
		associatedConf, err := utils.AssertType[*datamanager.AssociatedConfig](assocCfg)
		if err != nil {
			// This would only happen if there is a bug in resource graph
			return nil, err
		}

		res, err := resources.Lookup(name)
		if err != nil {
			c.logger.Warnw("datamanager failed to lookup resource from config", "error", err)
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
		c := collector
		wg.Add(1)
		goutils.ManagedGo(c.Close, wg.Done)
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
		c := collector
		wg.Add(1)
		goutils.ManagedGo(c.Flush, wg.Done)
	}
	wg.Wait()
}

func (c *Capture) maybeLogCollectorConfigChange(
	componentMethodMetadata resourceMethodMetadata,
	resConf datamanager.DataCaptureConfig,
) {
	prevFreqHz, ok := c.componentMethodFrequencyHz[componentMethodMetadata]

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

func defaultIfZeroVal[T comparable](val, defaultVal T) T {
	var zeroVal T
	if val == zeroVal {
		return defaultVal
	}
	return val
}

type captureDirPoller struct {
	logger logging.Logger

	mu         sync.Mutex
	captureDir string
	workers    utils.StoppableWorkers
}

func newCaptureDirPoller(logger logging.Logger) *captureDirPoller {
	return &captureDirPoller{logger: logger}
}

func (poller *captureDirPoller) reconfigure(captureDir string) {
	poller.mu.Lock()
	defer poller.mu.Unlock()
	if captureDir == poller.captureDir {
		return
	}

	poller.captureDir = captureDir
	if poller.workers != nil {
		poller.workers.Stop()
	}

	poller.workers = utils.NewStoppableWorkers(func(stopCtx context.Context) {
		t := time.NewTicker(captureDirSizeLogInterval)
		defer t.Stop()
		for {
			select {
			case <-stopCtx.Done():
				return
			case <-t.C:
				numFiles := countCaptureDirFiles(stopCtx, captureDir)
				if numFiles > minNumFiles {
					poller.logger.Infof("Capture dir contains %d files", numFiles)
				}
			}
		}
	})
}

func (poller *captureDirPoller) close() {
	poller.mu.Lock()
	defer poller.mu.Unlock()
	if poller.workers != nil {
		poller.workers.Stop()
	}
}
