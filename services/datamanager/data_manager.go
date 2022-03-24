package datamanager

import (
	"context"
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
type DataManager interface{}

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

type ComponentAttributes struct {
	Type              string            `json:"type"`
	Method            string            `json:"method"`
	CaptureIntervalMs int               `json:"capture_interval_ms"`
	AdditionalParams  map[string]string `json:"additional_params"`
}

// Config describes how to configure the service.
type Config struct {
	CaptureDir          string                         `json:"capture_dir"`
	ComponentAttributes map[string]ComponentAttributes `json:"component_attributes"`
}

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), "capture", ".viam")

// New returns a new data manager service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (DataManager, error) {
	dataManagerSvc := &DataManagerService{
		r:          r,
		logger:     logger,
		captureDir: viamCaptureDotDir,
		collectors: make(map[ComponentMethodMetadata]CollectorParams),
	}

	return dataManagerSvc, nil
}

type DataManagerService struct {
	r          robot.Robot
	logger     golog.Logger
	captureDir string
	collectors map[ComponentMethodMetadata]CollectorParams
}

type CollectorParams struct {
	Collector  data.Collector
	Attributes ComponentAttributes
	CaptureDir string
}

type ComponentMethodMetadata struct {
	ComponentName  string
	MethodMetadata data.MethodMetadata
}

func getDurationMs(captureIntervalMs int) time.Duration {
	return time.Duration(int64(time.Millisecond) * int64(captureIntervalMs))
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
func (svc *DataManagerService) initializeOrUpdateCollector(componentName string, attributes ComponentAttributes, updateCaptureDir bool) error {
	// Create component/method metadata to check if the collector exists.
	subtypeName := resource.SubtypeName(attributes.Type)
	metadata := data.MethodMetadata{
		Subtype:    subtypeName,
		MethodName: attributes.Method,
	}
	componentMetadata := ComponentMethodMetadata{
		ComponentName:  componentName,
		MethodMetadata: metadata,
	}
	if CollectorParams, ok := svc.collectors[componentMetadata]; ok {
		collector := CollectorParams.Collector
		previousAttributes := CollectorParams.Attributes

		// If the attributes have not changed, keep the current collector and update the target capture file if needed.
		if reflect.DeepEqual(previousAttributes, attributes) {
			if updateCaptureDir {
				targetFile, err := createDataCaptureFile(svc.captureDir, attributes.Type, componentName)
				if err != nil {
					return err
				}
				collector.SetTarget(targetFile)
			}
			return nil
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
		return err
	}

	// Get collector constructor for the component subtype and method.
	collectorConstructor := data.CollectorLookup(metadata)
	if collectorConstructor == nil {
		return errors.Errorf("failed to find collector for %s", metadata)
	}

	// Parameters to initialize collector.
	interval := getDurationMs(attributes.CaptureIntervalMs)
	targetFile, err := createDataCaptureFile(svc.captureDir, attributes.Type, componentName)
	if err != nil {
		return err
	}

	// Create a collector for this resource and method.
	collector, err := (*collectorConstructor)(res, componentName, interval, attributes.AdditionalParams, targetFile, svc.logger)
	if err != nil {
		return err
	}
	svc.collectors[componentMetadata] = CollectorParams{collector, attributes, svc.captureDir}

	// TODO: Handle err from Collect
	// TODO: Handle deletions. Currently only handling initial instantiation and updates.
	go collector.Collect()

	return nil
}

// Update updates the data manager service when the config has changed.
func (svc *DataManagerService) Update(ctx context.Context, config config.Service) error {
	svcConfig, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}
	updateCaptureDir := svc.captureDir != svcConfig.CaptureDir
	svc.captureDir = svcConfig.CaptureDir // TODO: Lock

	for componentName, attributes := range svcConfig.ComponentAttributes {
		if err := svc.initializeOrUpdateCollector(componentName, attributes, updateCaptureDir); err != nil {
			return err
		}
	}

	return nil
}
