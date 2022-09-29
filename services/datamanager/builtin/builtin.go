// Package builtin contains a service type that can be used to capture data from a robot's components.
package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
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
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterService(datamanager.Subtype, resource.DefaultModelName, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(ctx, r, c, logger)
		},
	})
	cType := config.ServiceType(datamanager.SubtypeName)
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
	resource.AddDefaultService(datamanager.Named(resource.DefaultModelName))
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
	Model              string               `json:"model"`
	Type               resource.SubtypeName `json:"type"`
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
	CaptureDir            string   `json:"capture_dir"`
	AdditionalSyncPaths   []string `json:"additional_sync_paths"`
	SyncIntervalMins      float64  `json:"sync_interval_mins"`
	CaptureDisabled       bool     `json:"capture_disabled"`
	ScheduledSyncDisabled bool     `json:"sync_disabled"`
}

// builtIn initializes and orchestrates data capture collectors for registered component/methods.
type builtIn struct {
	r                         robot.Robot
	logger                    golog.Logger
	captureDir                string
	captureDisabled           bool
	collectors                map[componentMethodMetadata]collectorAndConfig
	lock                      sync.Mutex
	backgroundWorkers         sync.WaitGroup
	updateCollectorsCancelFn  func()
	partID                    string
	waitAfterLastModifiedSecs int

	additionalSyncPaths []string
	syncDisabled        bool
	syncer              datasync.Manager
	syncerConstructor   datasync.ManagerConstructor
}

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), "capture", ".viam")

// NewBuiltIn returns a new data manager service for the given robot.
func NewBuiltIn(_ context.Context, r robot.Robot, _ config.Service, logger golog.Logger) (datamanager.Service, error) {
	// Set syncIntervalMins = -1 as we rely on initOrUpdateSyncer to instantiate a syncer
	// on first call to Update, even if syncIntervalMins value is 0, and the default value for int64 is 0.
	dataManagerSvc := &builtIn{
		r:                         r,
		logger:                    logger,
		captureDir:                viamCaptureDotDir,
		collectors:                make(map[componentMethodMetadata]collectorAndConfig),
		backgroundWorkers:         sync.WaitGroup{},
		lock:                      sync.Mutex{},
		additionalSyncPaths:       []string{},
		waitAfterLastModifiedSecs: 10,
		syncerConstructor:         datasync.NewDefaultManager,
	}

	return dataManagerSvc, nil
}

// Close releases all resources managed by data_manager.
func (svc *builtIn) Close(_ context.Context) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.closeCollectors()
	if svc.syncer != nil {
		svc.closeSyncer()
	}

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
	ComponentModel string
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
func (svc *builtIn) initializeOrUpdateCollector(attributes dataCaptureConfig) (*componentMethodMetadata, error) {
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
		collector.Close()
	}

	// Get the resource corresponding to the component subtype and name.
	resourceType := resource.NewSubtype(
		resource.ResourceNamespaceRDK,
		resource.ResourceTypeComponent,
		attributes.Type,
	)

	// Get the resource from the local or remote robot.
	var res interface{}
	if attributes.RemoteRobotName != "" {
		remoteRobot, exists := svc.r.RemoteByName(attributes.RemoteRobotName)
		if !exists {
			return nil, errors.Errorf("failed to find remote %s", attributes.RemoteRobotName)
		}
		res, err = remoteRobot.ResourceByName(resource.NameFromSubtype(resourceType, attributes.Name))
	} else {
		res, err = svc.r.ResourceByName(resource.NameFromSubtype(resourceType, attributes.Name))
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

	queue := datacapture.NewDeque(svc.captureDir, captureMetadata)

	// Create a collector for this resource and method.
	params := data.CollectorParams{
		ComponentName: attributes.Name,
		Interval:      interval,
		MethodParams:  methodParams,
		Target:        queue,
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

func (svc *builtIn) closeSyncer() {
	if svc.syncer != nil {
		fmt.Println("closing non nil syncer")
		// If previously we were syncing, close the old syncer and cancel the old updateCollectors goroutine.
		svc.syncer.Close()
		svc.syncer = nil
	}
}

func (svc *builtIn) initSyncer(cfg *config.Config) error {
	fmt.Println("initting syncer")
	syncer, err := svc.syncerConstructor(svc.logger, cfg)
	if err != nil {
		return errors.Wrap(err, "failed to initialize new syncer")
	}
	fmt.Println("done initting syncer")
	svc.syncer = syncer
	return nil
}

// TODO: when should I call this? Need to make sure it doesn't collide with the normal sync runs.
//       should basically cancel all previous ones before running this
func (svc *builtIn) syncPreviouslyCaptured() {
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
	svc.syncer.SyncFiles(previouslyCaptured)
}

// Sync performs a non-scheduled sync of the data in the capture directory.
func (svc *builtIn) Sync(_ context.Context) error {
	if svc.syncer == nil {
		return errors.New("called Sync on data manager service with nil syncer")
	}
	svc.syncDataCaptureFiles()
	svc.syncAdditionalSyncPaths()
	return nil
}

func (svc *builtIn) syncDataCaptureFiles() {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	queues := make([]*datacapture.Deque, len(svc.collectors))
	for _, collector := range svc.collectors {
		queues = append(queues, collector.Collector.GetTarget())
	}
	svc.syncer.Sync(queues)

	return
}

func (svc *builtIn) buildAdditionalSyncPaths() []string {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	var filepathsToSync []string
	// Loop through additional sync paths and add files from each to the syncer.
	for _, asp := range svc.additionalSyncPaths {
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
				// If a file was modified within the past svc.waitAfterLastModifiedSecs seconds, do not sync it (data
				// may still be being written).
				if diff := now.Sub(info.ModTime()); diff < (time.Duration(svc.waitAfterLastModifiedSecs) * time.Second) {
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
	// TODO: add back
	//svc.syncer.Sync(svc.buildAdditionalSyncPaths())
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
	if cfg.Cloud != nil {
		svc.partID = cfg.Cloud.ID
	}

	toggledCaptureOff := (svc.captureDisabled != svcConfig.CaptureDisabled) && svcConfig.CaptureDisabled
	svc.captureDisabled = svcConfig.CaptureDisabled
	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if toggledCaptureOff {
		svc.closeCollectors()
		svc.collectors = make(map[componentMethodMetadata]collectorAndConfig)
		return nil
	}

	allComponentAttributes, err := buildDataCaptureConfigs(cfg)
	if err != nil {
		return err
	}

	if len(allComponentAttributes) == 0 {
		svc.logger.Warn("Could not find any components with data_manager service configuration")
		return nil
	}

	svc.captureDir = svcConfig.CaptureDir

	// Initialize or add a collector based on changes to the component configurations.
	newCollectorMetadata := make(map[componentMethodMetadata]bool)
	for _, attributes := range allComponentAttributes {
		if !attributes.Disabled && attributes.CaptureFrequencyHz > 0 {
			componentMetadata, err := svc.initializeOrUpdateCollector(attributes)
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

	svc.syncDisabled = svcConfig.ScheduledSyncDisabled
	svc.closeSyncer()
	if !svc.syncDisabled {
		fmt.Println("sync sure is not disabled")
		// If the sync config has changed, update the syncer.
		svc.lock.Lock()
		svc.additionalSyncPaths = svcConfig.AdditionalSyncPaths
		svc.lock.Unlock()
		if err := svc.initSyncer(cfg); err != nil {
			return err
		}
		fmt.Println("syncing previously captured")
		svc.syncPreviouslyCaptured()
		fmt.Println("done syncing previously captured")
		queues := make([]*datacapture.Deque, len(svc.collectors))
		for _, c := range svc.collectors {
			queues = append(queues, c.Collector.GetTarget())
		}
		svc.syncer.Sync(queues)
	}

	return nil
}

func (svc *builtIn) uploadData(cancelCtx context.Context) {
	svc.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		svc.syncDataCaptureFiles()

		defer svc.backgroundWorkers.Done()
		//// time.Duration loses precision at low floating point values, so turn intervalMins to milliseconds.
		//intervalMillis := 60000.0 * intervalMins
		//ticker := time.NewTicker(time.Millisecond * time.Duration(intervalMillis))
		//defer ticker.Stop()
		//
		//for {
		//	if err := cancelCtx.Err(); err != nil {
		//		if !errors.Is(err, context.Canceled) {
		//			svc.logger.Errorw("data manager context closed unexpectedly", "error", err)
		//		}
		//		return
		//	}
		//	select {
		//	case <-cancelCtx.Done():
		//		return
		//	case <-ticker.C:
		//		svc.syncAdditionalSyncPaths()
		//	}
		//}
	})
}

func (svc *builtIn) startSyncBackgroundRoutine(intervalMins float64) {
	cancelCtx, fn := context.WithCancel(context.Background())
	svc.updateCollectorsCancelFn = fn
	svc.uploadData(cancelCtx)
}

// Get the config associated with the data manager service.
// Returns a boolean for whether a config is returned and an error if the
// config was incorrectly formatted.
func getServiceConfig(cfg *config.Config) (*Config, bool, error) {
	for _, c := range cfg.Services {
		// Compare service type and name.
		if c.ResourceName().ResourceSubtype == datamanager.SubtypeName {
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
					attrs.Model = c.Model
					attrs.Type = c.Type
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
					attrs.Type = name.ResourceSubtype
					attrs.RemoteRobotName = r.Name
					componentDataCaptureConfigs = append(componentDataCaptureConfigs, attrs)
				}
			}
		}
	}
	return componentDataCaptureConfigs, nil
}
