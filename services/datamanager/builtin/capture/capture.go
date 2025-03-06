// Package capture implements datacapture for the builtin datamanger
package capture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
)

// TODO: re-determine if queue size is optimal given we now support 10khz+ capture rates
// The Collector's queue should be big enough to ensure that .capture() is never blocked by the queue being
// written to disk. A default value of 250 was chosen because even with the fastest reasonable capture interval (1ms),
// this would leave 250ms for a (buffered) disk write before blocking, which seems sufficient for the size of
// writes this would be performing.
const defaultCaptureQueueSize = 250

const (
	// Default bufio.Writer buffer size in bytes.
	defaultCaptureBufferSize   = 4096
	defaultMongoDatabaseName   = "sensorData"
	defaultMongoCollectionName = "readings"
)

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
	logger logging.Logger
	clk    clock.Clock

	collectorsMu sync.Mutex
	collectors   collectors
	// captureDir is only stored on Capture so that we can detect when it changs
	captureDir string
	// maxCaptureFileSize is only stored on Capture so that we can detect when it changs
	maxCaptureFileSize int64
	mongoMU            sync.Mutex
	mongo              captureMongo
}

type captureMongo struct {
	// the struct members are protected by
	// mu and are either all nil or all non nil
	client     *mongo.Client
	collection *mongo.Collection
	config     *MongoConfig
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
		clk:        clock,
		logger:     logger,
		collectors: collectors{},
	}
}

func format(c datamanager.DataCaptureConfig) string {
	return fmt.Sprintf("datamanager.DataCaptureConfig{"+
		"Name: %s, Method: %s, CaptureFrequencyHz: %f, CaptureQueueSize: %d, AdditionalParams:	%v, Disabled: %t, Tags: %v, CaptureDirectory: %s}",
		c.Name, c.Method, c.CaptureFrequencyHz, c.CaptureQueueSize, c.AdditionalParams, c.Disabled, c.Tags, c.CaptureDirectory)
}

func (c *Capture) newCollectors(
	collectorConfigsByResource CollectorConfigsByResource,
	config Config,
	collection *mongo.Collection,
) collectors {
	// Initialize or add collectors based on changes to the component configurations.
	newCollectors := make(map[collectorMetadata]*collectorAndConfig)
	for res, cfgs := range collectorConfigsByResource {
		for _, cfg := range cfgs {
			md := newCollectorMetadata(cfg)

			// We only use service-level tags.
			cfg.Tags = config.Tags
			if cfg.Disabled {
				c.logger.Infof("collector disabled due to config `disabled` being true; collector: %s", md)
				continue
			}

			if cfg.CaptureFrequencyHz <= 0 {
				c.logger.Warnf("collector disabled due to config `capture_frequency_hz` being less than or equal to zero. collector: %s", md)
				continue
			}

			newCollectorAndConfig, err := c.initializeOrUpdateCollector(res, md, cfg, config, collection)
			if err != nil {
				c.logger.Warnw("failed to initialize or update collector",
					"error", err, "resource_name", res.Name(), "metadata", md, "data capture config", format(cfg))
				continue
			}
			newCollectors[md] = newCollectorAndConfig
		}
	}
	return newCollectors
}

func (c *Capture) mongoSetup(ctx context.Context, newConfig MongoConfig) *mongo.Collection {
	oldConfig := c.mongo.config
	if oldConfig != nil && oldConfig.Equal(newConfig) && c.mongo.client != nil {
		// if we have a client & the configs are equal, reuse the existing collection
		return c.mongo.collection
	}

	// We now know we want a mongo connection and that we either don't have one, or we have one
	// but the config has changed.
	// In either case we need to close all collectors and the client connection (if one exists),
	// create a new client & return the configured collection.
	c.closeNoMongoMutex(ctx)
	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	// Create a new client and connect to the server
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(newConfig.URI).SetServerAPIOptions(serverAPI))
	if err != nil {
		c.logger.Warn("failed to create mongo connection with mongo_capture_config.uri")
		return nil
	}
	database := defaultIfZeroVal(newConfig.Database, defaultMongoDatabaseName)
	collection := defaultIfZeroVal(newConfig.Collection, defaultMongoCollectionName)
	c.mongo = captureMongo{
		client:     client,
		collection: client.Database(database).Collection(collection),
		config:     &newConfig,
	}
	c.logger.Info("mongo client created")
	return c.mongo.collection
}

// mongoReconfigure shuts down the collectors when the mongo client is no longer being
// valid based on the new config and attempts to create a new mongo client when the new c
// config perscribes one.
// returns a *mongo.Collection when the new client is valid and nil when it is not.
func (c *Capture) mongoReconfigure(ctx context.Context, newConfig *MongoConfig) *mongo.Collection {
	c.mongoMU.Lock()
	defer c.mongoMU.Unlock()
	noClient := c.mongo.client == nil
	disabled := newConfig == nil || newConfig.URI == ""

	if noClient && disabled {
		// if we don't have a client and the new config
		// isn't asking for a mongo connection, no-op
		return nil
	}

	if disabled {
		// if we currently have a client, and the new config is disabled
		// call close to disconnect from mongo and close the collectors.
		// They will be recreated later during Reconfigure without a collection.
		c.closeNoMongoMutex(ctx)
		return nil
	}

	// If the config is enabled, setup mongo
	return c.mongoSetup(ctx, *newConfig)
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
		c.logger.Info("Capture Disabled")
		c.Close(ctx)
		return
	}

	if c.captureDir != config.CaptureDir {
		c.logger.Infof("capture_dir old: %s, new: %s", c.captureDir, config.CaptureDir)
	}

	if c.maxCaptureFileSize != config.MaximumCaptureFileSizeBytes {
		c.logger.Infof("maximum_capture_file_size_bytes old: %d, new: %d", c.maxCaptureFileSize, config.MaximumCaptureFileSizeBytes)
	}

	collection := c.mongoReconfigure(ctx, config.MongoConfig)
	newCollectors := c.newCollectors(collectorConfigsByResource, config, collection)
	// If a component/method has been removed from the config, close the collector.
	c.collectorsMu.Lock()
	for md, collAndConfig := range c.collectors {
		if _, present := newCollectors[md]; !present {
			c.logger.Infof("%s closing collector which is no longer in config", md.String())
			collAndConfig.Collector.Close()
		}
	}
	c.collectors = newCollectors
	c.collectorsMu.Unlock()
	c.captureDir = config.CaptureDir
	c.maxCaptureFileSize = config.MaximumCaptureFileSizeBytes
}

// Close closes the capture manager.
func (c *Capture) Close(ctx context.Context) {
	c.FlushCollectors()
	c.closeCollectors()
	c.mongoMU.Lock()
	defer c.mongoMU.Unlock()
	if c.mongo.client != nil {
		c.logger.Info("closing mongo connection")
		goutils.UncheckedError(c.mongo.client.Disconnect(ctx))
		c.mongo = captureMongo{}
	}
}

// closeNoMongoMutex exists for cases when we need to perform close actions in a function
// which is already holding the mongoMu.
func (c *Capture) closeNoMongoMutex(ctx context.Context) {
	c.FlushCollectors()
	c.closeCollectors()
	if c.mongo.client != nil {
		c.logger.Info("closing mongo connection")
		goutils.UncheckedError(c.mongo.client.Disconnect(ctx))
		c.mongo = captureMongo{}
	}
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (c *Capture) initializeOrUpdateCollector(
	res resource.Resource,
	md collectorMetadata,
	collectorConfig datamanager.DataCaptureConfig,
	config Config,
	collection *mongo.Collection,
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
		c.logger.Debugf("%s closing collector as config changed", md)
		storedCollectorAndConfig.Collector.Close()
	}

	// Get collector constructor for the component API and method.
	collectorConstructor := data.CollectorLookup(md.MethodMetadata)
	if collectorConstructor == nil {
		return nil, errors.Errorf("failed to find collector constructor for %s", md.MethodMetadata)
	}

	if collectorConfig.CaptureQueueSize < 0 {
		return nil, errors.Errorf("capture_queue_size can't be less than 0, current value: %d", collectorConfig.CaptureQueueSize)
	}

	if collectorConfig.CaptureBufferSize < 0 {
		return nil, errors.Errorf("capture_buffer_size can't be less than 0, current value: %d", collectorConfig.CaptureBufferSize)
	}

	metadataKey := generateMetadataKey(md.MethodMetadata.API.String(), md.MethodMetadata.MethodName)
	additionalParamKey, ok := metadataToAdditionalParamFields[metadataKey]
	if ok {
		if _, ok := collectorConfig.AdditionalParams[additionalParamKey]; !ok {
			return nil, errors.Errorf("failed to validate additional_params for %s, must supply %s",
				md.MethodMetadata.API, additionalParamKey)
		}
	}

	targetDir := targetDir(config.CaptureDir, collectorConfig)
	// Create a collector for this resource and method.
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, errors.Wrapf(err, "failed to create target directory %s with 700 file permissions", targetDir)
	}
	// Build metadata.
	captureMetadata, dataType := data.BuildCaptureMetadata(
		collectorConfig.Name.API,
		collectorConfig.Name.ShortName(),
		collectorConfig.Method,
		collectorConfig.AdditionalParams,
		methodParams,
		collectorConfig.Tags,
	)
	// Parameters to initialize collector.
	queueSize := defaultIfZeroVal(collectorConfig.CaptureQueueSize, defaultCaptureQueueSize)
	bufferSize := defaultIfZeroVal(collectorConfig.CaptureBufferSize, defaultCaptureBufferSize)
	collector, err := collectorConstructor(res, data.CollectorParams{
		MongoCollection: collection,
		DataType:        dataType,
		ComponentName:   collectorConfig.Name.ShortName(),
		ComponentType:   collectorConfig.Name.API.String(),
		MethodName:      collectorConfig.Method,
		Interval:        data.GetDurationFromHz(collectorConfig.CaptureFrequencyHz),
		MethodParams:    methodParams,
		Target:          data.NewCaptureBuffer(targetDir, captureMetadata, config.MaximumCaptureFileSizeBytes),
		// Set queue size to defaultCaptureQueueSize if it was not set in the config.
		QueueSize:  queueSize,
		BufferSize: bufferSize,
		Logger:     c.logger,
		Clock:      c.clk,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "constructor for collector %s failed with config: %s",
			md, collectorConfigDescription(collectorConfig, targetDir, config.MaximumCaptureFileSizeBytes, queueSize, bufferSize))
	}

	c.logger.Infof("collector initialized; collector: %s, config: %s",
		md, collectorConfigDescription(collectorConfig, targetDir, config.MaximumCaptureFileSizeBytes, queueSize, bufferSize))
	collector.Collect()

	return &collectorAndConfig{res, collector, collectorConfig}, nil
}

func collectorConfigDescription(
	collectorConfig datamanager.DataCaptureConfig,
	targetDir string,
	maximumCaptureFileSizeBytes int64,
	queueSize,
	bufferSize int,
) string {
	return fmt.Sprintf("[CaptureFrequencyHz: %f, Tags: %v, MaximumCaptureFileSize: %s, "+
		"CaptureBufferQueueSize: %d, CaptureBufferSize: %d, TargetDir: %s]",
		collectorConfig.CaptureFrequencyHz, collectorConfig.Tags, data.FormatBytesI64(maximumCaptureFileSizeBytes),
		queueSize, bufferSize, targetDir,
	)
}

func targetDir(captureDir string, collectorConfig datamanager.DataCaptureConfig) string {
	return data.CaptureFilePathWithReplacedReservedChars(
		filepath.Join(captureDir, collectorConfig.Name.API.String(),
			collectorConfig.Name.ShortName(), collectorConfig.Method))
}

// closeCollectors closes collectors.
func (c *Capture) closeCollectors() {
	var collectorsToClose []data.Collector
	var mds []collectorMetadata
	c.collectorsMu.Lock()
	for md, collectorAndConfig := range c.collectors {
		collectorsToClose = append(collectorsToClose, collectorAndConfig.Collector)
		mds = append(mds, md)
	}
	c.collectors = make(map[collectorMetadata]*collectorAndConfig)
	c.collectorsMu.Unlock()

	var wg sync.WaitGroup
	for i, collector := range collectorsToClose {
		tmp := collector
		md := mds[i]
		wg.Add(1)
		goutils.ManagedGo(
			func() {
				c.logger.Debugf("closing collector %s", md)
				tmp.Close()
				c.logger.Debugf("collector closed %s", md)
			}, wg.Done)
	}
	wg.Wait()
}

// FlushCollectors flushes collectors.
func (c *Capture) FlushCollectors() {
	var collectorsToFlush []data.Collector
	var mds []collectorMetadata
	c.collectorsMu.Lock()
	for md, collectorAndConfig := range c.collectors {
		collectorsToFlush = append(collectorsToFlush, collectorAndConfig.Collector)
		mds = append(mds, md)
	}
	c.collectorsMu.Unlock()

	var wg sync.WaitGroup
	for i, collector := range collectorsToFlush {
		tmp := collector
		md := mds[i]
		wg.Add(1)
		goutils.ManagedGo(func() {
			c.logger.Debugf("flushing collector %s", md)
			tmp.Flush()
			c.logger.Debugf("collector flushed %s", md)
		}, wg.Done)
	}
	wg.Wait()
}

func defaultIfZeroVal[T comparable](val, defaultVal T) T {
	var zeroVal T
	if val == zeroVal {
		return defaultVal
	}
	return val
}
