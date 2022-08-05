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
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	servicepb "go.viam.com/rdk/proto/api/service/datamanager/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.DataManagerService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterDataManagerServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &servicepb.DataManagerService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		Reconfigurable: WrapWithReconfigurable,
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

	resource.AddDefaultService(Name)
}

// Service defines what a Data Manager Service should expose to the users.
type Service interface {
	Sync(ctx context.Context) error
}

var (
	_ = Service(&reconfigurableDataManager{})
	_ = resource.Reconfigurable(&reconfigurableDataManager{})
	_ = goutils.ContextCloser(&reconfigurableDataManager{})
)

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

// Named is a helper for getting the named datamanager's typed resource name.
// RSDK-347 Implements datamanager's Named.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
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
	Type               resource.SubtypeName `json:"type"`
	Method             string               `json:"method"`
	CaptureFrequencyHz float32              `json:"capture_frequency_hz"`
	CaptureQueueSize   int                  `json:"capture_queue_size"`
	CaptureBufferSize  int                  `json:"capture_buffer_size"`
	AdditionalParams   map[string]string    `json:"additional_params"`
	Disabled           bool                 `json:"disabled"`
	RemoteRobotName    string               // Empty if this component is locally accessed
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

// dataManagerService initializes and orchestrates data capture collectors for registered component/methods.
type dataManagerService struct {
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

	// These are configuration variables used for configuring syncer and for checking if the config has changed
	// on subsequent updates.
	additionalSyncPaths []string
	syncDisabled        bool
	syncIntervalMins    float64
	// These two are direct pass through variables to syncer.
	uploadFunc        datasync.UploadFunc
	syncer            datasync.Manager
	syncerConstructor datasync.ManagerConstructor

	// Idea: replace above 3 with SyncerConstructor? And internal export SetSyncerConstructor
	//
}

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), "capture", ".viam")

// New returns a new data manager service for the given robot.
func New(_ context.Context, r robot.Robot, _ config.Service, logger golog.Logger) (Service, error) {
	// Set syncIntervalMins = -1 as we rely on initOrUpdateSyncer to instantiate a syncer
	// on first call to Update, even if syncIntervalMins value is 0, and the default value for int64 is 0.
	dataManagerSvc := &dataManagerService{
		r:                         r,
		logger:                    logger,
		captureDir:                viamCaptureDotDir,
		collectors:                make(map[componentMethodMetadata]collectorAndConfig),
		backgroundWorkers:         sync.WaitGroup{},
		lock:                      sync.Mutex{},
		syncIntervalMins:          -1,
		additionalSyncPaths:       []string{},
		waitAfterLastModifiedSecs: 10,
	}

	return dataManagerSvc, nil
}

// Close releases all resources managed by data_manager.
func (svc *dataManagerService) Close(_ context.Context) error {
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

func (svc *dataManagerService) closeCollectors() {
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

// Identifier for a particular collector: component name, component type, and method name.
type componentMethodMetadata struct {
	ComponentName  string
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
func (svc *dataManagerService) initializeOrUpdateCollector(
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
	// Build metadata.
	syncMetadata := datacapture.BuildCaptureMetadata(attributes.Type, attributes.Name,
		attributes.Method, attributes.AdditionalParams)

	if storedCollectorParams, ok := svc.collectors[componentMetadata]; ok {
		collector := storedCollectorParams.Collector
		previousAttributes := storedCollectorParams.Attributes

		// If the attributes have not changed, keep the current collector and update the target capture file if needed.
		if reflect.DeepEqual(previousAttributes, attributes) {
			if updateCaptureDir {
				targetFile, err := datacapture.CreateDataCaptureFile(svc.captureDir, syncMetadata)
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
	resourceType := resource.NewSubtype(
		resource.ResourceNamespaceRDK,
		resource.ResourceTypeComponent,
		attributes.Type,
	)

	// Get the resource from the local or remote robot.
	var res interface{}
	var err error
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
	targetFile, err := datacapture.CreateDataCaptureFile(svc.captureDir, syncMetadata)
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
	svc.lock.Lock()
	svc.collectors[componentMetadata] = collectorAndConfig{collector, attributes}
	svc.lock.Unlock()

	// TODO: Handle errors more gracefully.
	collector.Collect()

	return &componentMetadata, nil
}

func (svc *dataManagerService) initOrUpdateSyncer(_ context.Context, intervalMins float64, cfg *config.Config) error {
	// If user updates sync config while a sync is occurring, the running sync will be cancelled.
	// TODO DATA-235: fix that
	if svc.syncer != nil {
		// If previously we were syncing, close the old syncer and cancel the old updateCollectors goroutine.
		svc.syncer.Close()
		svc.syncer = nil
	}

	svc.cancelSyncBackgroundRoutine()

	// Kick off syncer if we're running it.
	if intervalMins > 0 {
		syncer, err := svc.syncerConstructor(svc.logger, svc.uploadFunc, cfg)
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
func (svc *dataManagerService) Sync(_ context.Context) error {
	if svc.syncer == nil {
		return errors.New("called Sync on data manager service with nil syncer")
	}
	svc.syncDataCaptureFiles()
	svc.syncAdditionalSyncPaths()
	return nil
}

func (svc *dataManagerService) syncDataCaptureFiles() {
	svc.lock.Lock()
	oldFiles := make([]string, 0, len(svc.collectors))
	for _, collector := range svc.collectors {
		// Create new target and set it.
		attributes := collector.Attributes
		syncMetadata := datacapture.BuildCaptureMetadata(attributes.Type, attributes.Name,
			attributes.Method, attributes.AdditionalParams)

		nextTarget, err := datacapture.CreateDataCaptureFile(svc.captureDir, syncMetadata)
		if err != nil {
			svc.logger.Errorw("failed to create new data capture file", "error", err)
		}
		oldFiles = append(oldFiles, collector.Collector.GetTarget().Name())
		collector.Collector.SetTarget(nextTarget)
	}
	svc.lock.Unlock()
	svc.syncer.Sync(oldFiles)
}

func (svc *dataManagerService) buildAdditionalSyncPaths() []string {
	svc.lock.Lock()
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
	svc.lock.Unlock()
	return filepathsToSync
}

// Syncs files under svc.additionalSyncPaths. If any of the directories do not exist, creates them.
func (svc *dataManagerService) syncAdditionalSyncPaths() {
	svc.syncer.Sync(svc.buildAdditionalSyncPaths())
}

// Update updates the data manager service when the config has changed.
func (svc *dataManagerService) Update(ctx context.Context, cfg *config.Config) error {
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

	if svc.syncerConstructor == nil {
		svc.syncerConstructor = datasync.NewDefaultManager
	}

	toggledCaptureOff := (svc.captureDisabled != svcConfig.CaptureDisabled) && svcConfig.CaptureDisabled
	svc.captureDisabled = svcConfig.CaptureDisabled
	// Service is disabled, so close all collectors and clear the map so we can instantiate new ones if we enable this service.
	if toggledCaptureOff {
		svc.closeCollectors()
		svc.collectors = make(map[componentMethodMetadata]collectorAndConfig)
		return nil
	}

	allComponentAttributes, err := getAllDataCaptureConfigs(cfg)
	if err != nil {
		return err
	}

	if len(allComponentAttributes) == 0 {
		svc.logger.Warn("Could not find any components with data_manager service configuration")
		return nil
	}

	toggledSync := svc.syncDisabled != svcConfig.ScheduledSyncDisabled
	svc.syncDisabled = svcConfig.ScheduledSyncDisabled
	toggledSyncOff := toggledSync && svc.syncDisabled
	toggledSyncOn := toggledSync && !svc.syncDisabled

	// If sync has been toggled on, sync previously captured files and update the capture directory.
	updateCaptureDir := (svc.captureDir != svcConfig.CaptureDir) || toggledSyncOn
	svc.captureDir = svcConfig.CaptureDir

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

func (svc *dataManagerService) uploadData(cancelCtx context.Context, intervalMins float64) {
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
				svc.syncDataCaptureFiles()
				svc.syncAdditionalSyncPaths()
			}
		}
	})
}

func (svc *dataManagerService) startSyncBackgroundRoutine(intervalMins float64) {
	cancelCtx, fn := context.WithCancel(context.Background())
	svc.updateCollectorsCancelFn = fn
	svc.uploadData(cancelCtx, intervalMins)
}

func (svc *dataManagerService) cancelSyncBackgroundRoutine() {
	if svc.updateCollectorsCancelFn != nil {
		svc.updateCollectorsCancelFn()
		svc.updateCollectorsCancelFn = nil
	}
}

type reconfigurableDataManager struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableDataManager) Sync(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Sync(ctx)
}

func (svc *reconfigurableDataManager) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return goutils.TryClose(ctx, svc.actual)
}

func (svc *reconfigurableDataManager) Update(ctx context.Context, resources *config.Config) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	updateableSvc, ok := svc.actual.(config.Updateable)
	if !ok {
		return errors.New("reconfigurable datamanager is not ConfigUpdateable")
	}
	return updateableSvc.Update(ctx, resources)
}

// Reconfigure replaces the old data manager service with a new data manager.
func (svc *reconfigurableDataManager) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableDataManager)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := goutils.TryClose(ctx, svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a data_manager as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("data_manager.Service", s)
	}

	if reconfigurable, ok := s.(*reconfigurableDataManager); ok {
		return reconfigurable, nil
	}

	return &reconfigurableDataManager{actual: svc}, nil
}

// Get the config associated with the data manager service.
// Returns a boolean for whether a config is returned and an error if the
// config was incorrectly formatted.
func getServiceConfig(cfg *config.Config) (*Config, bool, error) {
	for _, c := range cfg.Services {
		// Compare service type and name.
		if c.ResourceName() == Name {
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

// Get the component configs associated with the data manager service.
func getAllDataCaptureConfigs(cfg *config.Config) ([]dataCaptureConfig, error) {
	var componentDataCaptureConfigs []dataCaptureConfig
	for _, c := range cfg.Components {
		// Iterate over all component-level service configs of type data_manager.
		for _, componentSvcConfig := range c.ServiceConfig {
			if componentSvcConfig.ResourceName() == Name {
				attrs, err := getAttrsFromServiceConfig(componentSvcConfig)
				if err != nil {
					return componentDataCaptureConfigs, err
				}

				for _, attrs := range attrs.Attributes {
					attrs.Name = c.Name
					attrs.Type = c.Type
					componentDataCaptureConfigs = append(componentDataCaptureConfigs, attrs)
				}
			}
		}
	}

	for _, r := range cfg.Remotes {
		// Iterate over all remote-level service configs of type data_manager.
		for _, resourceSvcConfig := range r.ServiceConfig {
			if resourceSvcConfig.ResourceName() == Name {
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
