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
// - New
// - Reconfigure (any number of times)
// - Close (any number of times).
type Capture struct {
	logger          logging.Logger
	clk             clock.Clock
	fileCountLogger *fileCountLogger

	collectorsMu sync.Mutex
	collectors   collectors

	// captureDir is only stored on Capture so that we can detect when it changs
	captureDir string
	// maxCaptureFileSize is only stored on Capture so that we can detect when it changs
	maxCaptureFileSize int64
	// collectorFrequencyHz is only used to ensure we only log capture frequency values when they change in the config
	collectorFrequencyHz map[collectorMetadata]float32
}

type (
	collectors map[collectorMetadata]*collectorAndConfig
	// CollectorConfigsByResource describes the data capture configuration of the
	// resources/mathod pairs which data capture should capture data from.
	CollectorConfigsByResource map[resource.Resource][]datamanager.DataCaptureConfig
)

// Parameters stored for each collector.
type collectorAndConfig struct {
	Resource  resource.Resource
	Collector data.Collector
	Config    datamanager.DataCaptureConfig
}

// Identifier for a particular collector: component name, component model, component type,

// New creates a new capture manager.
func New(
	clock clock.Clock,
	logger logging.Logger,
) *Capture {
	return &Capture{
		clk:                  clock,
		logger:               logger,
		collectors:           collectors{},
		collectorFrequencyHz: make(map[collectorMetadata]float32),
		fileCountLogger:      newFileCountLogger(logger),
	}
}

func (c *Capture) newCollectors(collectorConfigsByResource CollectorConfigsByResource, config Config) collectors {
	// Initialize or add collectors based on changes to the component configurations.
	newCollectors := make(map[collectorMetadata]*collectorAndConfig)
	for res, cfgs := range collectorConfigsByResource {
		for _, cfg := range cfgs {
			md := newCollectorMetadata(cfg)
			// logging
			c.maybeLogCollectorConfigChange(md, cfg)
			// record to minimize duplicate logging
			c.collectorFrequencyHz[md] = cfg.CaptureFrequencyHz

			// We only use service-level tags.
			cfg.Tags = config.Tags
			if cfg.Disabled {
				c.logger.Debugf("%s disabled. config: %#v", md.String(), cfg)
				continue
			}

			if cfg.CaptureFrequencyHz <= 0 {
				msg := "%s disabled due to `capture_frequency_hz` being less than or equal to zero. config: %#v"
				c.logger.Debugf(msg, md.String(), cfg)
				continue
			}

			newCollectorAndConfig, err := c.initializeOrUpdateCollector(res, md, cfg, config)
			if err != nil {
				c.logger.Errorw("failed to initialize or update collector", "error", err)
				continue
			}
			c.logger.Debugf("%s initialized or updated with config: %#v", md.String(), cfg)
			newCollectors[md] = newCollectorAndConfig
		}
	}
	return newCollectors
}

// Reconfigure reconfigures Capture.
// It is only called by the builtin data manager.
func (c *Capture) Reconfigure(
	ctx context.Context,
	collectorConfigsByResource CollectorConfigsByResource,
	config Config,
) {
	c.logger.Debug("Reconfigure START")
	defer c.logger.Debug("Reconfigure END")
	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if config.CaptureDisabled {
		c.logger.Debug("Capture Disabled, flushing & shutting down collectors")
		c.Close()
		return
	}

	if c.captureDir != config.CaptureDir {
		c.fileCountLogger.reconfigure(config.CaptureDir)
	}

	newCollectors := c.newCollectors(collectorConfigsByResource, config)
	// If a component/method has been removed from the config, close the collector.
	c.collectorsMu.Lock()
	for md, collAndConfig := range c.collectors {
		if _, present := newCollectors[md]; !present {
			c.logger.Debugf("%s closing collector which is no longer in config", md.String())
			collAndConfig.Collector.Close()
		}
	}
	c.collectors = newCollectors
	c.collectorsMu.Unlock()
	c.captureDir = config.CaptureDir
	c.maxCaptureFileSize = config.MaximumCaptureFileSizeBytes
}

// Close closes the capture manager.
func (c *Capture) Close() {
	c.FlushCollectors()
	c.closeCollectors()
	c.fileCountLogger.close()
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (c *Capture) initializeOrUpdateCollector(
	res resource.Resource,
	md collectorMetadata,
	collectorConfig datamanager.DataCaptureConfig,
	config Config,
) (*collectorAndConfig, error) {
	// TODO(DATA-451): validate method params
	methodParams, err := protoutils.ConvertStringMapToAnyPBMap(collectorConfig.AdditionalParams)
	if err != nil {
		return nil, err
	}

	maxFileSizeChanged := c.maxCaptureFileSize != config.MaximumCaptureFileSizeBytes
	if storedCollectorAndConfig, ok := c.collectors[md]; ok {
		if storedCollectorAndConfig.Config.Equals(&collectorConfig) &&
			res == storedCollectorAndConfig.Resource &&
			!maxFileSizeChanged {
			// If the attributes have not changed, do nothing and leave the existing collector.
			return c.collectors[md], nil
		}
		// If the attributes have changed, close the existing collector.
		c.logger.Debugf("%s closing collector as config changed", md.String())
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
		if _, ok := collectorConfig.AdditionalParams[additionalParamKey]; !ok {
			return nil, errors.Errorf("failed to validate additional_params for %s, must supply %s",
				md.MethodMetadata.API.String(), additionalParamKey)
		}
	}

	targetDir := targetDir(config.CaptureDir, collectorConfig)
	// Create a collector for this resource and method.
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, err
	}
	// Build metadata.
	captureMetadata := data.BuildCaptureMetadata(
		collectorConfig.Name.API,
		collectorConfig.Name.ShortName(),
		collectorConfig.Method,
		collectorConfig.AdditionalParams,
		methodParams,
		collectorConfig.Tags,
	)
	// Parameters to initialize collector.
	collector, err := collectorConstructor(res, data.CollectorParams{
		ComponentName: collectorConfig.Name.ShortName(),
		Interval:      data.GetDurationFromHz(collectorConfig.CaptureFrequencyHz),
		MethodParams:  methodParams,
		Target:        data.NewCaptureBuffer(targetDir, captureMetadata, config.MaximumCaptureFileSizeBytes),
		// Set queue size to defaultCaptureQueueSize if it was not set in the config.
		QueueSize:  defaultIfZeroVal(collectorConfig.CaptureQueueSize, defaultCaptureQueueSize),
		BufferSize: defaultIfZeroVal(collectorConfig.CaptureBufferSize, defaultCaptureBufferSize),
		Logger:     c.logger,
		Clock:      c.clk,
	})
	if err != nil {
		return nil, err
	}

	collector.Collect()

	return &collectorAndConfig{res, collector, collectorConfig}, nil
}

func targetDir(captureDir string, collectorConfig datamanager.DataCaptureConfig) string {
	return data.CaptureFilePathWithReplacedReservedChars(
		filepath.Join(captureDir, collectorConfig.Name.API.String(),
			collectorConfig.Name.ShortName(), collectorConfig.Method))
}

// closeCollectors closes collectors.
func (c *Capture) closeCollectors() {
	var collectorsToClose []data.Collector
	c.collectorsMu.Lock()
	for _, collectorAndConfig := range c.collectors {
		collectorsToClose = append(collectorsToClose, collectorAndConfig.Collector)
	}
	c.collectors = make(map[collectorMetadata]*collectorAndConfig)
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

func (c *Capture) maybeLogCollectorConfigChange(
	componentMethodMetadata collectorMetadata,
	resConf datamanager.DataCaptureConfig,
) {
	prevFreqHz, ok := c.collectorFrequencyHz[componentMethodMetadata]

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

func defaultIfZeroVal[T comparable](val, defaultVal T) T {
	var zeroVal T
	if val == zeroVal {
		return defaultVal
	}
	return val
}
