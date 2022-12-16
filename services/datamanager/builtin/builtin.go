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
	goutils "go.viam.com/utils"

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

// Attributes to initialize the collector for a component or remote.
type dataCaptureConfig struct {
	Name               string               `json:"name"`
	Model              resource.Model       `json:"model"`
	Type               resource.Subtype     `json:"type"`
	Method             string               `json:"method"`
	CaptureFrequencyHz float32              `json:"capture_frequency_hz"`
	CaptureQueueSize   int                  `json:"capture_queue_size"`
	CaptureBufferSize  int                  `json:"capture_buffer_size"`
	AdditionalParams   map[string]string    `json:"additional_params"`
	Disabled           bool                 `json:"disabled"`
	RemoteRobotName    string               // Empty if this component is locally accessed
	Tags               []string             `json:"tags"`
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
	r                         robot.Robot
	logger                    golog.Logger
	syncLogger                golog.Logger
	captureDir                string
	captureDisabled           bool
	collectors                map[componentMethodMetadata]collectorAndConfig
	lock                      sync.Mutex
	backgroundWorkers         sync.WaitGroup
	updateCollectorsCancelFn  func()
	waitAfterLastModifiedSecs int

	additionalSyncPaths []string
	syncDisabled        bool
	syncIntervalMins    float64
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
		r:                         r,
		logger:                    logger,
		syncLogger:                syncLogger,
		captureDir:                viamCaptureDotDir,
		collectors:                make(map[componentMethodMetadata]collectorAndConfig),
		backgroundWorkers:         sync.WaitGroup{},
		lock:                      sync.Mutex{},
		syncIntervalMins:          -1,
		additionalSyncPaths:       []string{},
		waitAfterLastModifiedSecs: 10,
		syncerConstructor:         datasync.NewDefaultManager,
		modelManagerConstructor:   model.NewDefaultManager,
	}

	return dataManagerSvc, nil
}

// Close releases all resources managed by data_manager.
func (svc *builtIn) Close(_ context.Context) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.closeCollectors()
	if svc.syncer != nil {
		svc.syncer.Close()
	}

	svc.cancelSyncBackgroundRoutine()
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

// Parameters stored for each collector.
type collectorAndConfig struct {
	Collector  data.Collector
	Attributes dataCaptureConfig
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
	attributes dataCaptureConfig, updateCaptureDir bool) (
	*componentMethodMetadata, error,
) {
	// Create component/method metadata to check if the collector exists.
	metadata := data.MethodMetadata{
		Subtype:    attributes.Type,
		MethodName: attributes.Method,
	}

	componentMetadata := componentMethodMetadata{
		ComponentName:  attributes.Name,
		ComponentModel: attributes.Model,
		MethodParams:   fmt.Sprintf("%v", attributes.AdditionalParams),
		MethodMetadata: metadata,
	}
	// Build metadata.
	captureMetadata, err := datacapture.BuildCaptureMetadata(attributes.Type, attributes.Name,
		attributes.Model, attributes.Method, attributes.AdditionalParams, attributes.Tags)
	if err != nil {
		return nil, err
	}

	// TODO: DATA-451 https://viam.atlassian.net/browse/DATA-451 (validate method params)

	if storedCollectorParams, ok := svc.collectors[componentMetadata]; ok {
		collector := storedCollectorParams.Collector
		previousAttributes := storedCollectorParams.Attributes

		// If the attributes have not changed, keep the current collector and update the target capture file if needed.
		if reflect.DeepEqual(previousAttributes, attributes) {
			if updateCaptureDir {
				targetFile, err := datacapture.NewFile(svc.captureDir, captureMetadata)
				if err != nil {
					return nil, err
				}
				collector.SetTarget(targetFile)
			}
			return &componentMetadata, nil
		}

		// Otherwise, close the current collector and instantiate a new one below.
		collector.Close()
	}

	// Get the resource corresponding to the component subtype and name.

	// Get the resource from the local or remote robot.
	var res interface{}
	if attributes.RemoteRobotName != "" {
		remoteRobot, exists := svc.r.RemoteByName(attributes.RemoteRobotName)
		if !exists {
			return nil, errors.Errorf("failed to find remote %s", attributes.RemoteRobotName)
		}
		res, err = remoteRobot.ResourceByName(resource.NameFromSubtype(attributes.Type, attributes.Name))
	} else {
		res, err = svc.r.ResourceByName(resource.NameFromSubtype(attributes.Type, attributes.Name))
	}
	if err != nil {
		return nil, err
	}

	// Get collector constructor for the component subtype and method.
	collectorConstructor := data.CollectorLookup(metadata)
	if collectorConstructor == nil {
		return nil, errors.Errorf("failed to find collector for %s", metadata)
	}

	// Parameters to initialize collector.
	interval := getDurationFromHz(attributes.CaptureFrequencyHz)
	targetFile, err := datacapture.NewFile(svc.captureDir, captureMetadata)
	if err != nil {
		return nil, err
	}

	// Set queue size to defaultCaptureQueueSize if it was not set in the config.
	captureQueueSize := attributes.CaptureQueueSize
	if captureQueueSize == 0 {
		captureQueueSize = defaultCaptureQueueSize
	}

	captureBufferSize := attributes.CaptureBufferSize
	if captureBufferSize == 0 {
		captureBufferSize = defaultCaptureBufferSize
	}

	methodParams, err := protoutils.ConvertStringMapToAnyPBMap(attributes.AdditionalParams)
	if err != nil {
		return nil, err
	}

	// Create a collector for this resource and method.
	params := data.CollectorParams{
		ComponentName: attributes.Name,
		Interval:      interval,
		MethodParams:  methodParams,
		Target:        targetFile,
		QueueSize:     captureQueueSize,
		BufferSize:    captureBufferSize,
		Logger:        svc.logger,
	}
	collector, err := (*collectorConstructor)(res, params)
	if err != nil {
		return nil, err
	}
	svc.lock.Lock()
	svc.collectors[componentMetadata] = collectorAndConfig{collector, attributes}
	svc.lock.Unlock()

	// TODO: Handle errors more gracefully.
	collector.Collect()

	return &componentMetadata, nil
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
		MethodParams:   fmt.Sprintf("%v", attributes.AdditionalParams),
		MethodMetadata: metadata,
	}

	if storedCollectorParams, ok := svc.collectors[componentMetadata]; ok {
		collector := storedCollectorParams.Collector
		return collector, &componentMetadata
	}

	return nil, nil
}

func (svc *builtIn) initOrUpdateSyncer(_ context.Context, intervalMins float64, cfg *config.Config) error {
	// If user updates sync config while a sync is occurring, the running sync will be cancelled.
	// TODO DATA-235: fix that
	if svc.syncer != nil {
		// If previously we were syncing, close the old syncer and cancel the old updateCollectors goroutine.
		svc.syncer.Close()
		svc.syncer = nil
	}

	svc.cancelSyncBackgroundRoutine()

	// Kick off syncer if we're running it.
	if intervalMins > 0 && !svc.syncDisabled {
		syncer, err := svc.syncerConstructor(svc.syncLogger, cfg)
		if err != nil {
			return errors.Wrap(err, "failed to initialize new syncer")
		}
		svc.syncer = syncer

		// Sync existing files in captureDir.
		var previouslyCaptured []string
		//nolint
		_ = filepath.Walk(svc.captureDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			previouslyCaptured = append(previouslyCaptured, path)
			return nil
		})
		svc.syncer.Sync(previouslyCaptured)

		// Validate svc.additionSyncPaths all exist, and create them if not. Then sync files in svc.additionalSyncPaths.
		svc.syncer.Sync(svc.buildAdditionalSyncPaths())

		// Kick off background routine to periodically sync files.
		svc.startSyncBackgroundRoutine(intervalMins)
	}
	return nil
}

// Sync performs a non-scheduled sync of the data in the capture directory.
func (svc *builtIn) Sync(_ context.Context, extra map[string]interface{}) error {
	if svc.syncer == nil {
		return errors.New("called Sync on data manager service with nil syncer")
	}
	err := svc.syncDataCaptureFiles()
	if err != nil {
		return err
	}
	svc.syncAdditionalSyncPaths()
	return nil
}

func (svc *builtIn) syncDataCaptureFiles() error {
	svc.lock.Lock()
	oldFiles := make([]string, 0, len(svc.collectors))
	currCollectors := make(map[componentMethodMetadata]collectorAndConfig)
	for k, v := range svc.collectors {
		currCollectors[k] = v
	}
	svc.lock.Unlock()
	for _, collector := range currCollectors {
		// Create new target and set it.
		attributes := collector.Attributes
		captureMetadata, err := datacapture.BuildCaptureMetadata(attributes.Type, attributes.Name,
			attributes.Model, attributes.Method, attributes.AdditionalParams, attributes.Tags)
		if err != nil {
			return err
		}

		nextTarget, err := datacapture.NewFile(svc.captureDir, captureMetadata)
		if err != nil {
			return err
		}
		oldFiles = append(oldFiles, collector.Collector.GetTarget().GetPath())
		collector.Collector.SetTarget(nextTarget)
	}
	svc.syncer.Sync(oldFiles)
	return nil
}

func (svc *builtIn) buildAdditionalSyncPaths() []string {
	svc.lock.Lock()
	currAdditionalSyncPaths := svc.additionalSyncPaths
	waitAfterLastModified := svc.waitAfterLastModifiedSecs
	svc.lock.Unlock()

	var filepathsToSync []string
	// Loop through additional sync paths and add files from each to the syncer.
	for _, asp := range currAdditionalSyncPaths {
		// Check that additional sync paths directories exist. If not, create directories accordingly.
		if _, err := os.Stat(asp); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(asp, os.ModePerm)
			if err != nil {
				svc.logger.Errorw("data manager unable to create a directory specified as an additional sync path", "error", err)
			}
		} else {
			// Traverse all files in 'asp' directory and append them to a list of files to be synced.
			now := time.Now()
			//nolint
			_ = filepath.Walk(asp, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					return nil
				}
				// If a file was modified within the past waitAfterLastModifiedSecs seconds, do not sync it (data
				// may still be being written).
				if diff := now.Sub(info.ModTime()); diff < (time.Duration(waitAfterLastModified) * time.Second) {
					return nil
				}
				filepathsToSync = append(filepathsToSync, path)
				return nil
			})
		}
	}
	return filepathsToSync
}

// Syncs files under svc.additionalSyncPaths. If any of the directories do not exist, creates them.
func (svc *builtIn) syncAdditionalSyncPaths() {
	svc.syncer.Sync(svc.buildAdditionalSyncPaths())
}

// Update updates the data manager service when the config has changed.
func (svc *builtIn) Update(ctx context.Context, cfg *config.Config) error {
	svcConfig, ok, err := getServiceConfig(cfg)
	// Service is not in the config, has been removed from it, or is incorrectly formatted in the config.
	// Close any collectors.
	if !ok {
		svc.closeCollectors()
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

	toggledCaptureOff := (svc.captureDisabled != svcConfig.CaptureDisabled) && svcConfig.CaptureDisabled
	svc.captureDisabled = svcConfig.CaptureDisabled
	var allComponentAttributes []dataCaptureConfig

	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if toggledCaptureOff {
		svc.closeCollectors()
		svc.collectors = make(map[componentMethodMetadata]collectorAndConfig)
	} else {
		allComponentAttributes, err = buildDataCaptureConfigs(cfg)
		if err != nil {
			svc.logger.Warn(err.Error())
			return err
		}

		if len(allComponentAttributes) == 0 {
			svc.logger.Info("no components with data_manager service configuration")
		}
	}

	toggledSync := svc.syncDisabled != svcConfig.ScheduledSyncDisabled
	svc.syncDisabled = svcConfig.ScheduledSyncDisabled
	toggledSyncOff := toggledSync && svc.syncDisabled
	toggledSyncOn := toggledSync && !svc.syncDisabled

	// If sync has been toggled on, sync previously captured files and update the capture directory.
	captureDir := svcConfig.CaptureDir
	if captureDir == "" {
		captureDir = viamCaptureDotDir
	}
	updateCaptureDir := (svc.captureDir != captureDir) || toggledSyncOn
	svc.captureDir = captureDir

	// Stop syncing if newly disabled in the config.
	if toggledSyncOff {
		if err := svc.initOrUpdateSyncer(ctx, 0, cfg); err != nil {
			return err
		}
	} else if toggledSyncOn || (svcConfig.SyncIntervalMins != svc.syncIntervalMins) ||
		!reflect.DeepEqual(svcConfig.AdditionalSyncPaths, svc.additionalSyncPaths) {
		// If the sync config has changed, update the syncer.
		svc.lock.Lock()
		svc.additionalSyncPaths = svcConfig.AdditionalSyncPaths
		svc.lock.Unlock()
		svc.syncIntervalMins = svcConfig.SyncIntervalMins
		if err := svc.initOrUpdateSyncer(ctx, svcConfig.SyncIntervalMins, cfg); err != nil {
			return err
		}
	}

	// Initialize or add a collector based on changes to the component configurations.
	newCollectorMetadata := make(map[componentMethodMetadata]bool)
	for _, attributes := range allComponentAttributes {
		if !attributes.Disabled && attributes.CaptureFrequencyHz > 0 && !svc.captureDisabled {
			componentMetadata, err := svc.initializeOrUpdateCollector(
				attributes, updateCaptureDir)
			if err != nil {
				svc.logger.Errorw("failed to initialize or update collector", "error", err)
			} else {
				newCollectorMetadata[*componentMetadata] = true
			}
		} else if attributes.Disabled {
			// if disabled, make sure that it is closed, so it doesn't keep collecting data.
			collector, md := svc.getCollectorFromConfig(attributes)
			if collector != nil && md != nil {
				collector.Close()
				delete(svc.collectors, *md)
			}
		}
	}

	// If a component/method has been removed from the config, close the collector and remove it from the map.
	for componentMetadata, params := range svc.collectors {
		if _, present := newCollectorMetadata[componentMetadata]; !present {
			params.Collector.Close()
			delete(svc.collectors, componentMetadata)
		}
	}

	return nil
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
				err := svc.syncDataCaptureFiles()
				if err != nil {
					svc.logger.Errorw("data capture files failed to sync", "error", err)
				}
				// TODO DATA-660: There's a risk of deadlock where we're in this case when Close is called, which
				//                acquires svc.lock, which prevents this call from ever acquiring the lock/finishing.
				svc.syncAdditionalSyncPaths()
			}
		}
	})
}

func (svc *builtIn) startSyncBackgroundRoutine(intervalMins float64) {
	cancelCtx, fn := context.WithCancel(context.Background())
	svc.updateCollectorsCancelFn = fn
	svc.uploadData(cancelCtx, intervalMins)
}

func (svc *builtIn) cancelSyncBackgroundRoutine() {
	if svc.updateCollectorsCancelFn != nil {
		svc.updateCollectorsCancelFn()
		svc.updateCollectorsCancelFn = nil
	}
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
func buildDataCaptureConfigs(cfg *config.Config) ([]dataCaptureConfig, error) {
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
