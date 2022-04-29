// Package datamanager contains a service type that can be used to capture data from a robot's components.
package datamanager

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

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
type componentAttributes struct {
	Method             string            `json:"method"`
	CaptureFrequencyHz float32           `json:"capture_frequency_hz"`
	CaptureQueueSize   int               `json:"capture_queue_size"`
	CaptureBufferSize  int               `json:"capture_buffer_size"`
	AdditionalParams   map[string]string `json:"additional_params"`
}

// Config describes how to configure the service.
type Config struct {
	CaptureDir string `json:"capture_dir"`
}

// ComponentConfig describes how to configure the component/method within the service.
type ComponentMethodConfig struct {
	Name       string               `json:"name"`
	Type       resource.SubtypeName `json:"type"`
	Attributes componentAttributes
}

// TODO: Add configuration for remotes.

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), "capture", ".viam")

// New returns a new data manager service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (DataManager, error) {
	dataManagerSvc := &Service{
		r:          r,
		logger:     logger,
		captureDir: viamCaptureDotDir,
		collectors: make(map[componentMethodMetadata]collectorParams),
	}

	return dataManagerSvc, nil
}

// Service initializes and orchestrates data capture collectors for registered component/methods.
type Service struct {
	r          robot.Robot
	logger     golog.Logger
	captureDir string
	collectors map[componentMethodMetadata]collectorParams
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
	//
	if err := os.MkdirAll(fileDir, 0o700); err != nil {
		return nil, err
	}
	fileName := filepath.Join(fileDir, getFileTimestampName())
	return os.Create(fileName)
}

// Initialize a collector for the component/method or update it if it has previously been created.
// Return the component/method metadata which is used as a key in the collectors map.
func (svc *Service) initializeOrUpdateCollector(componentName string, componentType string, attributes componentAttributes, updateCaptureDir bool) (
	*componentMethodMetadata, error,
) {
	// Create component/method metadata to check if the collector exists.
	subtypeName := resource.SubtypeName(componentType)
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
				targetFile, err := createDataCaptureFile(svc.captureDir, componentType, componentName)
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
	targetFile, err := createDataCaptureFile(svc.captureDir, componentType, componentName)
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
		err := collector.Collect()
		if err != nil {
			svc.logger.Error(err.Error())
		}
	}()

	return &componentMetadata, nil
}

// Get the config associated with the data manager service.
func getServiceConfig(cfg *config.Config) (config.Service, error) {
	for _, c := range cfg.Services {
		if c.ResourceName() == Name {
			return c, nil
		}
	}
	return config.Service{}, errors.Errorf("could not find data_manager service")
}

// Get the component method configs associated with the data manager service.
func getComponentMethodConfigs(cfg *config.Config) ([]ComponentMethodConfig, error) {
	componentMethodConfigs := []ComponentMethodConfig{}
	for _, c := range cfg.Components {
		if svcConfig, ok := c.ServiceConfig["data_manager"]; ok {
			for _, methodConfiguration := range (svcConfig["capture_methods"]).([]interface{}) {
				// Encode json map into struct.
				// TODO: Can this work similar to ConvertedAttributes which automatically infer the
				// struct directly from the `json` fields? Tried something similar and didn't work.
				componentAttrs := componentAttributes{}
				marshaled, _ := json.Marshal(methodConfiguration)
				err := json.Unmarshal(marshaled, &componentAttrs)
				if err != nil {
					return nil, err
				}

				// Create a componentMethodConfig from the method attributes.
				componentMethodConfig := ComponentMethodConfig{c.Name, c.Type, componentAttrs}
				componentMethodConfigs = append(componentMethodConfigs, componentMethodConfig)
			}
		}
	}
	return componentMethodConfigs, nil
}

// Update updates the data manager service when the config has changed.
func (svc *Service) Update(ctx context.Context, cfg *config.Config) error {
	svcConfig, err := getServiceConfig(cfg)
	if err != nil {
		return err
	}
	convertedSvcConfig, ok := svcConfig.ConvertedAttributes.(*Config)
	if !ok {
		return utils.NewUnexpectedTypeError(convertedSvcConfig, svcConfig.ConvertedAttributes)
	}
	updateCaptureDir := svc.captureDir != convertedSvcConfig.CaptureDir
	svc.captureDir = convertedSvcConfig.CaptureDir

	componentMethodConfigs, err := getComponentMethodConfigs(cfg)
	if err != nil {
		return err
	}
	if len(componentMethodConfigs) == 0 {
		return errors.Errorf("could not find and components with data_manager service configuration")
	}

	// Initialize or add a collector based on changes to the component configurations.
	newCollectorMetadata := make(map[componentMethodMetadata]bool)
	for _, componentMethodConfig := range componentMethodConfigs {
		if componentMethodConfig.Attributes.CaptureFrequencyHz > 0 {
			componentMetadata, err := svc.initializeOrUpdateCollector(
				componentMethodConfig.Name, (string)(componentMethodConfig.Type),
				componentMethodConfig.Attributes, updateCaptureDir)
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
