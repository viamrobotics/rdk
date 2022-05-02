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

// SyncQueuePath is the directory under which files are queued while they are waiting to be synced to the cloud.
var SyncQueuePath = filepath.Join(os.Getenv("HOME"), "sync_queue", ".viam")

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
type componentAttributes struct {
	Type               string            `json:"type"`
	Method             string            `json:"method"`
	CaptureFrequencyHz float32           `json:"capture_frequency_hz"`
	CaptureQueueSize   int               `json:"capture_queue_size"`
	CaptureBufferSize  int               `json:"capture_buffer_size"`
	AdditionalParams   map[string]string `json:"additional_params"`
}

// Config describes how to configure the service.
type Config struct {
	CaptureDir          string                         `json:"capture_dir"`
	AdditionalSyncPaths []string                       `json:"additional_sync_paths"`
	SyncIntervalMins    int                            `json:"sync_interval_mins"`
	ComponentAttributes map[string]componentAttributes `json:"component_attributes"`
}

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), "capture", ".viam")

// New returns a new data manager service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (DataManager, error) {
	dataManagerSvc := &Service{
		r:                 r,
		logger:            logger,
		captureDir:        viamCaptureDotDir,
		collectors:        make(map[componentMethodMetadata]collectorParams),
		backgroundWorkers: sync.WaitGroup{},
		lock:              sync.Mutex{},
	}

	return dataManagerSvc, nil
}

// Close releases all resources managed by data_manager.
func (svc *Service) Close(ctx context.Context) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	for _, collector := range svc.collectors {
		collector.Collector.Close()
	}
	if svc.syncer != nil {
		svc.updateCollectorsCancelFn()
		svc.syncer.Close()
	}
	svc.backgroundWorkers.Wait()
	return nil
}

// Service initializes and orchestrates data capture collectors for registered component/methods.
type Service struct {
	r          robot.Robot
	logger     golog.Logger
	captureDir string
	collectors map[componentMethodMetadata]collectorParams
	syncer     syncManager

	lock                     sync.Mutex
	backgroundWorkers        sync.WaitGroup
	updateCollectorsCancelFn func()
}

// Parameters stored for each collector.
type collectorParams struct {
	Collector  data.Collector
	Attributes componentAttributes
	CaptureDir string
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
func createDataCaptureFile(captureDir string, subtypeName string, componentName string) (*os.File, error) {
	fileDir := filepath.Join(captureDir, subtypeName, componentName)
	if err := os.MkdirAll(fileDir, 0o700); err != nil {
		return nil, err
	}
	fileName := filepath.Join(fileDir, getFileTimestampName())
	return os.Create(fileName)
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (svc *Service) initializeOrUpdateCollector(componentName string, attributes componentAttributes, updateCaptureDir bool) (
	*componentMethodMetadata, error,
) {
	// Create component/method metadata to check if the collector exists.
	subtypeName := resource.SubtypeName(attributes.Type)
	metadata := data.MethodMetadata{
		Subtype:    subtypeName,
		MethodName: attributes.Method,
	}
	componentMetadata := componentMethodMetadata{
		ComponentName:  componentName,
		MethodMetadata: metadata,
	}
	if storedCollectorParams, ok := svc.collectors[componentMetadata]; ok {
		collector := storedCollectorParams.Collector
		previousAttributes := storedCollectorParams.Attributes

		// If the attributes have not changed, keep the current collector and update the target capture file if needed.
		if reflect.DeepEqual(previousAttributes, attributes) {
			if updateCaptureDir {
				targetFile, err := createDataCaptureFile(svc.captureDir, attributes.Type, componentName)
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
		subtypeName,
	)
	res, err := svc.r.ResourceByName(resource.NameFromSubtype(subtype, componentName))
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
	targetFile, err := createDataCaptureFile(svc.captureDir, attributes.Type, componentName)
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
	collector, err := (*collectorConstructor)(
		res, componentName, interval, attributes.AdditionalParams,
		targetFile, captureQueueSize, captureBufferSize, svc.logger)
	if err != nil {
		return nil, err
	}
	svc.collectors[componentMetadata] = collectorParams{collector, attributes, svc.captureDir}

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
		svc.queueCapturedData(cancelCtx, intervalMins)
		svc.syncer = newSyncer(SyncQueuePath, svc.logger, svc.captureDir)
		svc.syncer.Start()
	}
}

// Update updates the data manager service when the config has changed.
func (svc *Service) Update(ctx context.Context, config config.Service) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	svcConfig, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}
	updateCaptureDir := svc.captureDir != svcConfig.CaptureDir
	svc.captureDir = svcConfig.CaptureDir
	// nolint:contextcheck
	svc.initOrUpdateSyncer(svcConfig.SyncIntervalMins)

	// Initialize or add a collector based on changes to the config.
	newCollectorMetadata := make(map[componentMethodMetadata]bool)
	for componentName, attributes := range svcConfig.ComponentAttributes {
		if attributes.CaptureFrequencyHz > 0 {
			componentMetadata, err := svc.initializeOrUpdateCollector(componentName, attributes, updateCaptureDir)
			if err != nil {
				return err
			}
			newCollectorMetadata[*componentMetadata] = true
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

func (svc *Service) queueCapturedData(cancelCtx context.Context, intervalMins int) {
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
				files := make([]string, 0, len(svc.collectors))
				for _, collector := range svc.collectors {
					files = append(files, collector.Collector.GetTarget().Name())
				}
				if err := svc.syncer.Enqueue(files); err != nil {
					svc.logger.Errorw("failed to move files to sync queue", "error", err)
				}
				return
			case <-ticker.C:
				oldFiles := make([]string, 0, len(svc.collectors))
				svc.lock.Lock()
				for component, collector := range svc.collectors {
					// Create new target and set it.
					nextTarget, err := createDataCaptureFile(svc.captureDir, collector.Attributes.Type, component.ComponentName)
					if err != nil {
						svc.logger.Errorw("failed to create new data capture file", "error", err)
					}
					oldFiles = append(oldFiles, collector.Collector.GetTarget().Name())
					collector.Collector.SetTarget(nextTarget)
				}
				svc.lock.Unlock()
				if err := svc.syncer.Enqueue(oldFiles); err != nil {
					svc.logger.Errorw("failed to move files to sync queue", "error", err)
				}
			}
		}
	})
}
