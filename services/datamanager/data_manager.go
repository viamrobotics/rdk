package datamanager

import (
	"context"
	"os"
	"path/filepath"

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

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("data_manager")

// Subtype is a constant that identifies the data manager service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Config describes how to configure the service.
type Config struct {
	CaptureDir          string                         `json:"capture_dir"`
	ComponentAttributes map[string]config.AttributeMap `json:"component_attributes"`
}

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

// FromRobot retrieves the data manager service of a robot.
func FromRobot(r robot.Robot) (DataManager, error) {
	resource, err := r.ResourceByName(Name)
	if err != nil {
		return nil, err
	}
	svc, ok := resource.(DataManager)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("datamanager.Service", resource)
	}
	return svc, nil
}

// Name is the DataManager's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	return nil
}

var viamCaptureDotDir = filepath.Join(os.Getenv("HOME"), "capture", ".viam")

// New returns a new data manager service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (DataManager, error) {
	dataManagerSvc := &DataManagerService{
		r:          r,
		logger:     logger,
		captureDir: viamCaptureDotDir,
		collectors: make(map[ComponentMethodMetadata]CollectorAndAttributes),
	}

	return dataManagerSvc, nil
}

type CollectorAndAttributes struct {
	Collector  data.Collector
	Attributes config.AttributeMap
}

type DataManagerService struct {
	r          robot.Robot
	logger     golog.Logger
	captureDir string
	collectors map[ComponentMethodMetadata]CollectorAndAttributes
}

type ComponentMethodMetadata struct {
	ComponentName  string
	MethodMetadata data.MethodMetadata
}

func getDurationMs(captureIntervalMs int) time.Duration {
	return time.Duration(int64(time.Millisecond) * int64(captureIntervalMs))
}

func getFileTimestampName() string {
	// RFC3339Nano is a standard time format e.g. 2006-01-02T15:04:05Z07:00.
	return time.Now().Format(time.RFC3339Nano)
}

func createDataCaptureFile(logger golog.Logger, captureDir string, subtypeName string, componentName string) (*os.File, error) {
	fileDir := filepath.Join(captureDir, subtypeName, componentName)
	if err := os.MkdirAll(fileDir, 0o700); err != nil {
		return nil, err
	}
	fileName := filepath.Join(fileDir, getFileTimestampName())
	logger.Info("Writing to ", fileName) // TODO: remove this and logger param before submit
	return os.Create(fileName)
}

func (svc *DataManagerService) initializeOrUpdateCollector(componentName string, attributes config.AttributeMap) error {
	// Create component/method metadata to check if the collector exists.
	subtypeName := resource.SubtypeName(attributes.String("type"))
	metadata := data.MethodMetadata{
		Subtype:    subtypeName,
		MethodName: attributes.String("method"),
	}
	componentMetadata := ComponentMethodMetadata{
		ComponentName:  componentName,
		MethodMetadata: metadata,
	}
	// if collectorAndAttributes, ok := svc.collectors[componentMetadata]; ok {

	// }

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
	interval := getDurationMs(attributes.Int("capture_interval_ms", 0))
	targetFile, err := createDataCaptureFile(svc.logger, svc.captureDir, attributes.String("type"), componentName)
	if err != nil {
		return err
	}

	additionalParams := map[string]string{} // TODO: support method-specific params.
	logger := svc.logger                    // Or new logger for each?

	// Create a collector for this resource and method.
	collector, err := (*collectorConstructor)(res, componentName, interval, additionalParams, targetFile, logger)
	if err != nil {
		return err
	}
	svc.collectors[componentMetadata] = CollectorAndAttributes{collector, attributes}

	// TODO: Handle err from Collect
	// TODO: Handle updates and deletions. Currently only handling initial instantiation.
	go collector.Collect()

	return nil
}

// Update updates the data manager service when the config has changed.
func (svc *DataManagerService) Update(ctx context.Context, config config.Service) error {
	svcConfig, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}
	svc.captureDir = svcConfig.CaptureDir // TODO: Lock

	for componentName, attributes := range svcConfig.ComponentAttributes {
		svc.logger.Info("initializing ", componentName) // TODO: remove before submit
		if err := svc.initializeOrUpdateCollector(componentName, attributes); err != nil {
			return err
		}
	}

	return nil
}
