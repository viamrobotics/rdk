// Package datamanager contains a service type that can be used to capture data from a robot's components.
package datamanager

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})
	cType := config.ServiceType(SubtypeName)
	config.RegisterServiceAttributeMapConverter(cType, func(attributes config.AttributeMap) (interface{}, error) {
		var conf Config
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(attributes); err != nil {
			return nil, err
		}
		return &conf, nil
	}, &Config{})
}

// DataManager defines what a Data Manager Service should be able to do.
type DataManager interface { // TODO: Add synchronize.
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("data_manager")

// Subtype is a constant that identifies the data manager service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the DataManager's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// The Collector's queue should be big enough to ensure that .capture() is never blocked by the queue being
// written to disk. A default value of 250 was chosen because even with the fastest reasonable capture interval (1ms),
// this would leave 250ms for a (buffered) disk write before blocking, which seems sufficient for the size of
// writes this would be performing.
const defaultCaptureQueueSize = 250

// Default bufio.Writer buffer size in bytes.
const defaultCaptureBufferSize = 4096

// Attributes to initialize the collector for a component.
type dataCaptureConfig struct {
	Name               string               `json:"name"`
	Type               resource.SubtypeName `json:"type"`
	Method             string               `json:"method"`
	CaptureFrequencyHz float32              `json:"capture_frequency_hz"`
	CaptureQueueSize   int                  `json:"capture_queue_size"`
	CaptureBufferSize  int                  `json:"capture_buffer_size"`
	AdditionalParams   map[string]string    `json:"additional_params"`
	Disabled           bool                 `json:"disabled"`
}

type dataCaptureConfigs struct {
	Attributes []dataCaptureConfig `json:"capture_methods"`
}

// Config describes how to configure the service.
type Config struct {
	CaptureDir          string   `json:"capture_dir"`
	AdditionalSyncPaths []string `json:"additional_sync_paths"`
	SyncIntervalMins    int      `json:"sync_interval_mins"`
	Disabled            bool     `json:"disabled"`
}

// TODO(https://viam.atlassian.net/browse/DATA-157): Add configuration for remotes.

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), "capture", ".viam")

// New returns a new data manager service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (DataManager, error) {
	dataManagerSvc := &Service{
		r:                 r,
		logger:            logger,
		captureDir:        viamCaptureDotDir,
		collectors:        make(map[componentMethodMetadata]collectorAndConfig),
		backgroundWorkers: sync.WaitGroup{},
		lock:              sync.Mutex{},
	}

	return dataManagerSvc, nil
}

// Close releases all resources managed by data_manager.
func (svc *Service) Close(ctx context.Context) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.closeCollectors()
	if svc.syncer != nil {
		svc.updateCollectorsCancelFn()
		svc.syncer.Close()
	}
	svc.backgroundWorkers.Wait()
	return nil
}

func (svc *Service) closeCollectors() {
	wg := sync.WaitGroup{}
	for _, collector := range svc.collectors {
		currCollector := collector
		wg.Add(1)
		go func() {
			currCollector.Collector.Close()
			wg.Done()
		}()
	}
	wg.Wait()
}

// Service initializes and orchestrates data capture collectors for registered component/methods.
type Service struct {
	r          robot.Robot
	logger     golog.Logger
	captureDir string
	collectors map[componentMethodMetadata]collectorAndConfig
	syncer     syncManager

	lock                     sync.Mutex
	backgroundWorkers        sync.WaitGroup
	updateCollectorsCancelFn func()
}

// Parameters stored for each collector.
type collectorAndConfig struct {
	Collector  data.Collector
	Attributes dataCaptureConfig
}

// Identifier for a particular collector: component name, component type, and method name.
type componentMethodMetadata struct {
	ComponentName  string
	MethodMetadata data.MethodMetadata
}

// Get time.Duration from hz.
func getDurationFromHz(captureFrequencyHz float32) time.Duration {
	return time.Second / time.Duration(captureFrequencyHz)
}

// Create a filename based on the current time.
func getFileTimestampName() string {
	// RFC3339Nano is a standard time format e.g. 2006-01-02T15:04:05Z07:00.
	return time.Now().Format(time.RFC3339Nano)
}

// Create a timestamped file within the given capture directory.
func createDataCaptureFile(captureDir string, subtypeName resource.SubtypeName, componentName string) (*os.File, error) {
	fileDir := filepath.Join(captureDir, string(subtypeName), componentName)
	if err := os.MkdirAll(fileDir, 0o700); err != nil {
		return nil, err
	}
	fileName := filepath.Join(fileDir, getFileTimestampName())
	//nolint:gosec
	return os.Create(fileName)
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (svc *Service) initializeOrUpdateCollector(
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
		MethodMetadata: metadata,
	}
	if storedCollectorParams, ok := svc.collectors[componentMetadata]; ok {
		collector := storedCollectorParams.Collector
		previousAttributes := storedCollectorParams.Attributes

		// If the attributes have not changed, keep the current collector and update the target capture file if needed.
		if reflect.DeepEqual(previousAttributes, attributes) {
			if updateCaptureDir {
				targetFile, err := createDataCaptureFile(
					svc.captureDir, attributes.Type, attributes.Name)
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
	subtype := resource.NewSubtype(
		resource.ResourceNamespaceRDK,
		resource.ResourceTypeComponent,
		attributes.Type,
	)
	res, err := svc.r.ResourceByName(resource.NameFromSubtype(subtype, attributes.Name))
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
	targetFile, err := createDataCaptureFile(svc.captureDir, attributes.Type, attributes.Name)
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

	// Create a collector for this resource and method.
	params := data.CollectorParams{
		ComponentName: attributes.Name,
		Interval:      interval,
		MethodParams:  attributes.AdditionalParams,
		Target:        targetFile,
		QueueSize:     captureQueueSize,
		BufferSize:    captureBufferSize,
		Logger:        svc.logger,
	}
	collector, err := (*collectorConstructor)(res, params)
	if err != nil {
		return nil, err
	}
	svc.collectors[componentMetadata] = collectorAndConfig{collector, attributes}

	// TODO: Handle errors more gracefully.
	go func() {
		if err := collector.Collect(); err != nil {
			svc.logger.Error(err.Error())
		}
	}()

	return &componentMetadata, nil
}

func (svc *Service) initOrUpdateSyncer(intervalMins int) {
	if svc.syncer != nil {
		// If previously we were syncing, close the old syncer and cancel the old updateCollectors goroutine.
		svc.updateCollectorsCancelFn()
		svc.syncer.Close()
		svc.backgroundWorkers.Wait()
		svc.syncer = nil
		svc.updateCollectorsCancelFn = nil
	}

	// Init a new syncer if we are still syncing.
	if intervalMins > 0 {
		cancelCtx, fn := context.WithCancel(context.Background())
		svc.updateCollectorsCancelFn = fn
		svc.syncer = newSyncer(svc.logger)
		var previouslyCaptured []string
		//nolint
		_ = filepath.Walk(svc.captureDir, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			previouslyCaptured = append(previouslyCaptured, path)
			return nil
		})
		svc.syncer.Sync(previouslyCaptured)
		svc.uploadCapturedData(cancelCtx, intervalMins)
	}
}

// Get the config associated with the data manager service.
func getServiceConfig(cfg *config.Config) (config.Service, bool) {
	for _, c := range cfg.Services {
		// Compare service type and name.
		if c.ResourceName() == Name {
			return c, true
		}
	}
	return config.Service{}, false
}

// Get the component configs associated with the data manager service.
func getAllDataCaptureConfigs(cfg *config.Config) ([]dataCaptureConfig, error) {
	componentDataCaptureConfigs := []dataCaptureConfig{}
	for _, c := range cfg.Components {
		// Iterate over all component-level service configs of type data_manager.
		for _, componentSvcConfig := range c.ServiceConfig {
			if componentSvcConfig.ResourceName() == Name {
				var attrs dataCaptureConfigs
				configs, err := config.TransformAttributeMapToStruct(&attrs, componentSvcConfig.Attributes)
				if err != nil {
					return componentDataCaptureConfigs, err
				}
				convertedConfigs, ok := configs.(*dataCaptureConfigs)
				if !ok {
					return componentDataCaptureConfigs, utils.NewUnexpectedTypeError(convertedConfigs, configs)
				}

				// Add the method configuration to the result.
				for _, attrs := range convertedConfigs.Attributes {
					attrs.Name = c.Name
					attrs.Type = c.Type
					componentDataCaptureConfigs = append(componentDataCaptureConfigs, attrs)
				}
			}
		}
	}
	return componentDataCaptureConfigs, nil
}

// Update updates the data manager service when the config has changed.
func (svc *Service) Update(ctx context.Context, cfg *config.Config) error {
	c, ok := getServiceConfig(cfg)
	// Service is not in the config or has been removed from it. Close any collectors.
	if !ok {
		svc.closeCollectors()
		return nil
	}

	svcConfig, ok := c.ConvertedAttributes.(*Config)

	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if svcConfig.Disabled {
		svc.closeCollectors()
		svc.collectors = make(map[componentMethodMetadata]collectorAndConfig)
		return nil
	}

	if !ok {
		return utils.NewUnexpectedTypeError(svcConfig, c.ConvertedAttributes)
	}
	updateCaptureDir := svc.captureDir != svcConfig.CaptureDir
	svc.captureDir = svcConfig.CaptureDir

	allComponentAttributes, err := getAllDataCaptureConfigs(cfg)
	if err != nil {
		return err
	}

	if len(allComponentAttributes) == 0 {
		svc.logger.Warn("Could not find any components with data_manager service configuration")
		return nil
	}

	// nolint:contextcheck
	svc.initOrUpdateSyncer(svcConfig.SyncIntervalMins)

	// Initialize or add a collector based on changes to the component configurations.
	newCollectorMetadata := make(map[componentMethodMetadata]bool)
	for _, attributes := range allComponentAttributes {
		if !attributes.Disabled && attributes.CaptureFrequencyHz > 0 {
			componentMetadata, err := svc.initializeOrUpdateCollector(
				attributes, updateCaptureDir)
			if err != nil {
				svc.logger.Errorw("failed to initialize or update collector", "error", err)
			} else {
				newCollectorMetadata[*componentMetadata] = true
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

func (svc *Service) uploadCapturedData(cancelCtx context.Context, intervalMins int) {
	svc.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer svc.backgroundWorkers.Done()
		ticker := time.NewTicker(time.Minute * time.Duration(intervalMins))
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
				oldFiles := make([]string, 0, len(svc.collectors))
				for _, collector := range svc.collectors {
					// Create new target and set it.
					nextTarget, err := createDataCaptureFile(svc.captureDir, collector.Attributes.Type, collector.Attributes.Name)
					if err != nil {
						svc.logger.Errorw("failed to create new data capture file", "error", err)
					}
					oldFiles = append(oldFiles, collector.Collector.GetTarget().Name())
					collector.Collector.SetTarget(nextTarget)
				}
				svc.lock.Unlock()
				svc.syncer.Sync(oldFiles)
			}
		}
	})
}
