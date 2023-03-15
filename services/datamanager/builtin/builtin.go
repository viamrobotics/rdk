// Package builtin contains a service type that can be used to capture data from a robot's components.
package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/services/datamanager/model"
	"go.viam.com/rdk/utils"
	goutils "go.viam.com/utils"
	"golang.org/x/exp/slices"
)

func init() {
	registry.RegisterService(datamanager.Subtype, resource.DefaultServiceModel, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(ctx, r, c, logger)
		},
	})
	config.RegisterServiceAttributeMapConverter(datamanager.Subtype, resource.DefaultServiceModel,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		}, &Config{},
	)
	resource.AddDefaultService(datamanager.Named(resource.DefaultServiceName))
}

// TODO: re-determine if queue size is optimal given we now support 10khz+ capture rates
// The Collector's queue should be big enough to ensure that .capture() is never blocked by the queue being
// written to disk. A default value of 250 was chosen because even with the fastest reasonable capture interval (1ms),
// this would leave 250ms for a (buffered) disk write before blocking, which seems sufficient for the size of
// writes this would be performing.
const defaultCaptureQueueSize = 250

// Default bufio.Writer buffer size in bytes.
const defaultCaptureBufferSize = 4096

var errCaptureDirectoryConfigurationDisabled = errors.New("changing the capture directory is prohibited in this environment")

// Config to initialize the collector for a component or remote.
type dataCaptureConfig struct {
	Name               string            `json:"name"`
	Model              resource.Model    `json:"model"`
	Type               resource.Subtype  `json:"type"`
	Method             string            `json:"method"`
	CaptureFrequencyHz float32           `json:"capture_frequency_hz"`
	CaptureQueueSize   int               `json:"capture_queue_size"`
	CaptureBufferSize  int               `json:"capture_buffer_size"`
	AdditionalParams   map[string]string `json:"additional_params"`
	Disabled           bool              `json:"disabled"`
	RemoteRobotName    string            // Empty if this component is locally accessed
	Tags               []string          `json:"tags"`
	CaptureDirectory   string            `json:"capture_directory"`
}

func (c *dataCaptureConfig) Equals(other *dataCaptureConfig) bool {
	return c.Name == other.Name &&
		c.Model == other.Model &&
		c.Type == other.Type &&
		c.Method == other.Method &&
		c.CaptureFrequencyHz == other.CaptureFrequencyHz &&
		c.CaptureQueueSize == other.CaptureQueueSize &&
		c.CaptureBufferSize == other.CaptureBufferSize &&
		c.Disabled == other.Disabled &&
		c.RemoteRobotName == other.RemoteRobotName &&
		slices.Compare(c.Tags, other.Tags) == 0 &&
		reflect.DeepEqual(c.AdditionalParams, other.AdditionalParams) &&
		c.CaptureDirectory == other.CaptureDirectory
}

type dataCaptureConfigs struct {
	Attributes []dataCaptureConfig `json:"capture_methods"`
}

// Config describes how to configure the service.
type Config struct {
	CaptureDir            string         `json:"capture_dir"`
	AdditionalSyncPaths   []string       `json:"additional_sync_paths"`
	SyncIntervalMins      float64        `json:"sync_interval_mins"`
	CaptureDisabled       bool           `json:"capture_disabled"`
	ScheduledSyncDisabled bool           `json:"sync_disabled"`
	ModelsToDeploy        []*model.Model `json:"models_on_robot"`
}

// builtIn initializes and orchestrates data capture collectors for registered component/methods.
type builtIn struct {
	generic.Unimplemented
	r                           robot.Robot
	cfg                         *config.Config
	logger                      golog.Logger
	syncLogger                  golog.Logger
	captureDir                  string
	captureDisabled             bool
	collectors                  map[componentMethodMetadata]*collectorAndConfig
	lock                        sync.Mutex
	backgroundWorkers           sync.WaitGroup
	waitAfterLastModifiedMillis int

	additionalSyncPaths []string
	syncDisabled        bool
	syncIntervalMins    float64
	syncRoutineCancelFn context.CancelFunc
	syncer              datasync.Manager
	syncerConstructor   datasync.ManagerConstructor

	modelManager            model.Manager
	modelManagerConstructor model.ManagerConstructor
}

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), ".viam", "capture")

// NewBuiltIn returns a new data manager service for the given robot.
func NewBuiltIn(_ context.Context, r robot.Robot, _ config.Service, logger golog.Logger) (datamanager.Service, error) {
	var syncLogger golog.Logger
	// Create a production logger for its smart sampling defaults, since if many collectors or upload retries
	// fail at once, the syncer will otherwise spam logs.
	productionLogger, err := zap.NewProduction()
	if err != nil {
		syncLogger = logger // Default to the provided logger.
	} else {
		syncLogger = productionLogger.Sugar()
	}

	// Set syncIntervalMins = -1 as we rely on initOrUpdateSyncer to instantiate a syncer
	// on first call to Update, even if syncIntervalMins value is 0, and the default value for int64 is 0.
	dataManagerSvc := &builtIn{
		r:                           r,
		logger:                      logger,
		syncLogger:                  syncLogger,
		captureDir:                  viamCaptureDotDir,
		collectors:                  make(map[componentMethodMetadata]*collectorAndConfig),
		backgroundWorkers:           sync.WaitGroup{},
		lock:                        sync.Mutex{},
		syncIntervalMins:            0,
		additionalSyncPaths:         []string{},
		waitAfterLastModifiedMillis: 10000,
		syncerConstructor:           datasync.NewDefaultManager,
		modelManagerConstructor:     model.NewDefaultManager,
	}

	return dataManagerSvc, nil
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
	wg := sync.WaitGroup{}
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
	wg := sync.WaitGroup{}
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
	Config    dataCaptureConfig
}

// Identifier for a particular collector: component name, component model, component type,
// method parameters, and method name.
type componentMethodMetadata struct {
	ComponentName  string
	ComponentModel resource.Model
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
	md componentMethodMetadata,
	config dataCaptureConfig) (
	*collectorAndConfig, error,
) {
	// Build metadata.
	captureMetadata, err := datacapture.BuildCaptureMetadata(config.Type, config.Name,
		config.Model, config.Method, config.AdditionalParams, config.Tags)
	if err != nil {
		return nil, err
	}

	// TODO: DATA-451 https://viam.atlassian.net/browse/DATA-451 (validate method params)

	if storedCollectorAndConfig, ok := svc.collectors[md]; ok {
		if storedCollectorAndConfig.Config.Equals(&config) {
			// If the attributes have not changed, do nothing and leave the existing collector.
			return svc.collectors[md], nil
		} else {
			// If the attributes have changed, close the existing collector.
			storedCollectorAndConfig.Collector.Close()
		}
	}

	// Build and start a new collector for this component/method.
	// Get the resource from the local or remote robot.
	var res interface{}
	if config.RemoteRobotName != "" {
		remoteRobot, exists := svc.r.RemoteByName(config.RemoteRobotName)
		if !exists {
			return nil, errors.Errorf("failed to find remote %s", config.RemoteRobotName)
		}
		res, err = remoteRobot.ResourceByName(resource.NameFromSubtype(config.Type, config.Name))
	} else {
		res, err = svc.r.ResourceByName(resource.NameFromSubtype(config.Type, config.Name))
	}
	if err != nil {
		return nil, err
	}

	// Get collector constructor for the component subtype and method.
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
		ComponentName: config.Name,
		Interval:      interval,
		MethodParams:  methodParams,
		Target:        datacapture.NewBuffer(targetDir, captureMetadata),
		QueueSize:     captureQueueSize,
		BufferSize:    captureBufferSize,
		Logger:        svc.logger,
	}
	collector, err := (*collectorConstructor)(res, params)
	if err != nil {
		return nil, err
	}
	collector.Collect()

	return &collectorAndConfig{collector, config}, nil
}

func (svc *builtIn) closeSyncer() {
	if svc.syncer != nil {
		// If previously we were syncing, close the old syncer and cancel the old updateCollectors goroutine.
		svc.syncer.Close()
		svc.syncer = nil
	}
}

func (svc *builtIn) initSyncer(cfg *config.Config) error {
	syncer, err := svc.syncerConstructor(svc.logger, cfg, svc.waitAfterLastModifiedMillis)
	if err != nil {
		return errors.Wrap(err, "failed to initialize new syncer")
	}
	svc.syncer = syncer
	return nil
}

// getCollectorFromConfig returns the collector and metadata that is referenced based on specific config atrributes
func (svc *builtIn) getCollectorFromConfig(attributes dataCaptureConfig) (data.Collector, *componentMethodMetadata) {
	// Create component/method metadata to check if the collector exists.
	metadata := data.MethodMetadata{
		Subtype:    attributes.Type,
		MethodName: attributes.Method,
	}

	componentMetadata := componentMethodMetadata{
		ComponentName:  attributes.Name,
		ComponentModel: attributes.Model,
		MethodMetadata: metadata,
		MethodParams:   fmt.Sprintf("%v", attributes.AdditionalParams),
	}

	if storedCollectorParams, ok := svc.collectors[componentMetadata]; ok {
		collector := storedCollectorParams.Collector
		return collector, &componentMetadata
	}

	return nil, nil
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
func (svc *builtIn) Sync(_ context.Context, _ map[string]interface{}) error {
	svc.lock.Lock()
	if svc.syncer == nil {
		err := svc.initSyncer(svc.cfg)
		if err != nil {
			svc.lock.Unlock()
			return err
		}
	}

	svc.flushCollectors()
	svc.syncer.SyncDirectory(svc.captureDir)
	svc.syncAdditionalSyncPaths()
	svc.lock.Unlock()
	return nil
}

// Syncs files under svc.additionalSyncPaths. If any of the directories do not exist, creates them.
func (svc *builtIn) syncAdditionalSyncPaths() {
	for _, dir := range svc.additionalSyncPaths {
		svc.syncer.SyncDirectory(dir)
	}
}

// Update updates the data manager service when the config has changed.
func (svc *builtIn) Update(ctx context.Context, cfg *config.Config) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.cfg = cfg

	svcConfig, ok, err := getServiceConfig(cfg)
	// Service is not in the config, has been removed from it, or is incorrectly formatted in the config.
	// Close any collectors.
	if !ok {
		svc.closeCollectors()
		svc.closeSyncer()
		return err
	}

	// Check that we have models to download and appropriate credentials.
	if len(svcConfig.ModelsToDeploy) > 0 && cfg.Cloud != nil {
		if svc.modelManager == nil {
			modelManager, err := svc.modelManagerConstructor(svc.logger, cfg)
			if err != nil {
				return errors.Wrap(err, "failed to initialize new modelManager")
			}
			svc.modelManager = modelManager
		}

		// Download models from models_on_robot.
		modelsToDeploy := svcConfig.ModelsToDeploy
		errorChannel := make(chan error, len(modelsToDeploy))
		go svc.modelManager.DownloadModels(cfg, modelsToDeploy, errorChannel)
		if len(errorChannel) != 0 {
			var errMsgs []string
			for err := range errorChannel {
				errMsgs = append(errMsgs, err.Error())
			}
			return errors.New(strings.Join(errMsgs[:], ", "))
		}
	}

	dcConfigs, err := buildDataCaptureConfigs(cfg, svcConfig.CaptureDir)
	if err != nil {
		return err
	}

	if cfg.LimitConfigurableDirectories && svcConfig.CaptureDir != "" && svcConfig.CaptureDir != viamCaptureDotDir {
		return errCaptureDirectoryConfigurationDisabled
	}
	if svcConfig.CaptureDir != "" {
		svc.captureDir = svcConfig.CaptureDir
	}
	svc.captureDisabled = svcConfig.CaptureDisabled
	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if svc.captureDisabled {
		svc.closeCollectors()
		svc.collectors = make(map[componentMethodMetadata]*collectorAndConfig)
	}

	// Initialize or add collectors based on changes to the component configurations.
	newCollectors := make(map[componentMethodMetadata]*collectorAndConfig)
	if !svc.captureDisabled {
		for _, attributes := range dcConfigs {
			if !attributes.Disabled && attributes.CaptureFrequencyHz > 0 {
				// Create component/method metadata to check if the collector exists.
				methodMetadata := data.MethodMetadata{
					Subtype:    attributes.Type,
					MethodName: attributes.Method,
				}

				componentMethodMetadata := componentMethodMetadata{
					ComponentName:  attributes.Name,
					ComponentModel: attributes.Model,
					MethodMetadata: methodMetadata,
					MethodParams:   fmt.Sprintf("%v", attributes.AdditionalParams),
				}

				newCollectorAndConfig, err := svc.initializeOrUpdateCollector(componentMethodMetadata, attributes)
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

	svc.syncDisabled = svcConfig.ScheduledSyncDisabled
	svc.syncIntervalMins = svcConfig.SyncIntervalMins
	svc.additionalSyncPaths = svcConfig.AdditionalSyncPaths

	// TODO DATA-861: this means that the ticker is reset everytime we call Update with sync enabled, regardless of
	//      whether or not the interval has changed. We should not do that.
	svc.cancelSyncScheduler()
	if !svc.syncDisabled && svc.syncIntervalMins != 0.0 {
		if svc.syncer == nil {
			if err := svc.initSyncer(cfg); err != nil {
				return err
			}
		}
		svc.startSyncScheduler(svc.syncIntervalMins)
	} else {
		svc.closeSyncer()
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
	}
}

func (svc *builtIn) uploadData(cancelCtx context.Context, intervalMins float64) {
	svc.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer svc.backgroundWorkers.Done()
		// time.Duration loses precision at low floating point values, so turn intervalMins to milliseconds.
		intervalMillis := 60000.0 * intervalMins
		ticker := time.NewTicker(time.Millisecond * time.Duration(intervalMillis))
		defer ticker.Stop()

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
			case <-ticker.C:
				svc.lock.Lock()
				if svc.syncer != nil {
					svc.flushCollectors()
					svc.syncer.SyncDirectory(svc.captureDir)
					svc.syncAdditionalSyncPaths()
				}
				svc.lock.Unlock()
			}
		}
	})
}

// Get the config associated with the data manager service.
// Returns a boolean for whether a config is returned and an error if the
// config was incorrectly formatted.
func getServiceConfig(cfg *config.Config) (*Config, bool, error) {
	for _, c := range cfg.Services {
		// Compare service type and name.
		if c.ResourceName().Subtype == datamanager.Subtype && c.ConvertedAttributes != nil {
			svcConfig, ok := c.ConvertedAttributes.(*Config)
			// Incorrect configuration is an error.
			if !ok {
				return &Config{}, false, utils.NewUnexpectedTypeError(svcConfig, c.ConvertedAttributes)
			}
			return svcConfig, true, nil
		}
	}

	// Data Manager Service is not in the config, which is not an error.
	return &Config{}, false, nil
}

func getAttrsFromServiceConfig(resourceSvcConfig config.ResourceLevelServiceConfig) (dataCaptureConfigs, error) {
	var attrs dataCaptureConfigs
	configs, err := config.TransformAttributeMapToStruct(&attrs, resourceSvcConfig.Attributes)
	if err != nil {
		return dataCaptureConfigs{}, err
	}
	convertedConfigs, ok := configs.(*dataCaptureConfigs)
	if !ok {
		return dataCaptureConfigs{}, utils.NewUnexpectedTypeError(convertedConfigs, configs)
	}
	return *convertedConfigs, nil
}

// Build the component configs associated with the data manager service.
func buildDataCaptureConfigs(cfg *config.Config, captureDir string) ([]dataCaptureConfig, error) {
	var componentDataCaptureConfigs []dataCaptureConfig
	for _, c := range cfg.Components {
		// Iterate over all component-level service configs of type data_manager.
		for _, componentSvcConfig := range c.ServiceConfig {
			if componentSvcConfig.Type == datamanager.SubtypeName {
				attrs, err := getAttrsFromServiceConfig(componentSvcConfig)
				if err != nil {
					return componentDataCaptureConfigs, err
				}
				for _, attrs := range attrs.Attributes {
					attrs.Name = c.Name
					// TODO(PRODUCT-266): move this to using triplets
					attrs.Model = c.Model
					attrs.Type = c.ResourceName().Subtype // Using this instead of c.API to guarantee it's backward compatible
					attrs.CaptureDirectory = captureDir
					componentDataCaptureConfigs = append(componentDataCaptureConfigs, attrs)
				}
			}
		}
	}

	for _, r := range cfg.Remotes {
		// Iterate over all remote-level service configs of type data_manager.
		for _, resourceSvcConfig := range r.ServiceConfig {
			if resourceSvcConfig.Type == datamanager.SubtypeName {
				attrs, err := getAttrsFromServiceConfig(resourceSvcConfig)
				if err != nil {
					return componentDataCaptureConfigs, err
				}

				for _, attrs := range attrs.Attributes {
					name, err := resource.NewFromString(attrs.Name)
					if err != nil {
						return componentDataCaptureConfigs, err
					}
					attrs.Name = name.Name
					attrs.Type = name.Subtype
					attrs.RemoteRobotName = r.Name
					componentDataCaptureConfigs = append(componentDataCaptureConfigs, attrs)
				}
			}
		}
	}
	return componentDataCaptureConfigs, nil
}
