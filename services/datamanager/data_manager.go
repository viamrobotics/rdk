// Package datamanager contains a service type that can be used to capture data from a robot's components.
// For more information, see the [data management service docs].
//
// [data management service docs]: https://docs.viam.com/services/data/
package datamanager

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"

	servicepb "go.viam.com/api/service/datamanager/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	resource.RegisterAPIWithAssociation(
		API,
		resource.APIRegistration[Service]{
			RPCServiceServerConstructor: NewRPCServiceServer,
			RPCServiceHandler:           servicepb.RegisterDataManagerServiceHandlerFromEndpoint,
			RPCServiceDesc:              &servicepb.DataManagerService_ServiceDesc,
			RPCClient:                   NewClientFromConn,
			MaxInstance:                 resource.DefaultMaxInstance,
		},
		resource.AssociatedConfigRegistration[*AssociatedConfig]{
			AttributeMapConverter: newAssociatedConfig,
		},
	)
}

// Service defines what a Data Manager Service should expose to the users.
// For more information, see the [data management service docs].
//
// Sync example:
//
//	// Sync data stored on the machine to the cloud.
//	err := data.Sync(context.Background(), nil)
//
// For more information, see the [Sync method docs].
//
// [data management service docs]: https://docs.viam.com/data-ai/capture-data/capture-sync/
// [Sync method docs]: https://docs.viam.com/dev/reference/apis/services/data/#sync
type Service interface {
	resource.Resource
	// Sync will sync data stored on the machine to the cloud.
	Sync(ctx context.Context, extra map[string]interface{}) error
}

// SubtypeName is the name of the type of service.
const SubtypeName = "data_manager"

// API is a variable that identifies the data manager service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named datamanager's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromDependencies is a helper for getting the named data manager service from a collection of dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Service, error) {
	return resource.FromDependencies[Service](deps, Named(name))
}

// FromRobot is a helper for getting the named data manager service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// NamesFromRobot is a helper for getting all data manager services from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// AssociatedConfig specify a list of methods to capture on resources and implements the resource.AssociatedConfig interface.
type AssociatedConfig struct {
	CaptureMethods []DataCaptureConfig `json:"capture_methods"`
}

func newAssociatedConfig(attributes utils.AttributeMap) (*AssociatedConfig, error) {
	md, err := json.Marshal(attributes)
	if err != nil {
		return nil, err
	}
	var conf AssociatedConfig
	if err := json.Unmarshal(md, &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

// Equals describes if an DataCaptureConfigs is equal to a given AssociatedConfig.
func (ac *AssociatedConfig) Equals(other resource.AssociatedConfig) bool {
	ac2, err := utils.AssertType[*AssociatedConfig](other)
	if err != nil {
		return false
	}
	if len(ac.CaptureMethods) != len(ac2.CaptureMethods) {
		return false
	}
	// naively iterate over the list of capture methods and determine if they are the same
	// note that two lists with capture methods [a, b] and [b, a] will not be equal as they are out of order
	for i := 0; i < len(ac.CaptureMethods); i++ {
		if !ac.CaptureMethods[i].Equals(&ac2.CaptureMethods[i]) {
			return false
		}
	}
	return true
}

// UpdateResourceNames allows the caller to modify the resource names of data capture in place.
func (ac *AssociatedConfig) UpdateResourceNames(updater func(old resource.Name) resource.Name) {
	for idx := range ac.CaptureMethods {
		ac.CaptureMethods[idx].Name = updater(ac.CaptureMethods[idx].Name)
	}
}

// Link associates an AssociatedConfig to a specific resource model (e.g. builtin data capture).
func (ac *AssociatedConfig) Link(conf *resource.Config) {
	if len(ac.CaptureMethods) == 0 {
		return
	}

	// infer name from first index in CaptureMethods
	name := ac.CaptureMethods[0].Name
	captureMethodCopies := make([]DataCaptureConfig, 0, len(ac.CaptureMethods))
	for _, method := range ac.CaptureMethods {
		methodCopy := method
		captureMethodCopies = append(captureMethodCopies, methodCopy)
	}
	if conf.AssociatedAttributes == nil {
		conf.AssociatedAttributes = make(map[resource.Name]resource.AssociatedConfig)
	}
	conf.AssociatedAttributes[name] = &AssociatedConfig{CaptureMethods: captureMethodCopies}
}

// DataCaptureConfig is used to initialize a collector for a component or remote.
type DataCaptureConfig struct {
	Name               resource.Name     `json:"name"`
	Method             string            `json:"method"`
	CaptureFrequencyHz float32           `json:"capture_frequency_hz"`
	CaptureQueueSize   int               `json:"capture_queue_size"`
	CaptureBufferSize  int               `json:"capture_buffer_size"`
	AdditionalParams   map[string]string `json:"additional_params"`
	Disabled           bool              `json:"disabled"`
	Tags               []string          `json:"tags,omitempty"`
	CaptureDirectory   string            `json:"capture_directory"`
}

// Equals checks if one capture config is equal to another.
func (c *DataCaptureConfig) Equals(other *DataCaptureConfig) bool {
	return c.Name.String() == other.Name.String() &&
		c.Method == other.Method &&
		c.CaptureFrequencyHz == other.CaptureFrequencyHz &&
		c.CaptureQueueSize == other.CaptureQueueSize &&
		c.CaptureBufferSize == other.CaptureBufferSize &&
		c.Disabled == other.Disabled &&
		slices.Compare(c.Tags, other.Tags) == 0 &&
		reflect.DeepEqual(c.AdditionalParams, other.AdditionalParams) &&
		c.CaptureDirectory == other.CaptureDirectory
}

// ShouldSyncKey is a special key we use within a modular sensor to pass a boolean
// that indicates to the datamanager whether or not we want to sync.
var ShouldSyncKey = "should_sync"

// CreateShouldSyncReading is a helper for creating the expected reading for a modular sensor
// that passes a bool to the datamanager to indicate whether or not we want to sync.
func CreateShouldSyncReading(toSync bool) map[string]interface{} {
	readings := map[string]interface{}{}
	readings[ShouldSyncKey] = toSync
	return readings
}
