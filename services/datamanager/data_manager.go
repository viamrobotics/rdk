// Package datamanager contains a service type that can be used to capture data from a robot's components.
package datamanager

import (
	"context"
	"encoding/json"
	"reflect"

	servicepb "go.viam.com/api/service/datamanager/v1"
	"golang.org/x/exp/slices"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           servicepb.RegisterDataManagerServiceHandlerFromEndpoint,
		RPCServiceDesc:              &servicepb.DataManagerService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
		MaxInstance:                 resource.DefaultMaxInstance,
	})
	config.RegisterResourceAssociationConfigConverter(
		Subtype,
		func(attributes utils.AttributeMap) (interface{}, error) {
			md, err := json.Marshal(attributes)
			if err != nil {
				return nil, err
			}
			var conf DataCaptureConfigs
			if err := json.Unmarshal(md, &conf); err != nil {
				return nil, err
			}
			return &conf, nil
		},
		func(resName resource.Name, resAssociation interface{}) error {
			capConf, err := utils.AssertType[*DataCaptureConfigs](resAssociation)
			if err != nil {
				return err
			}
			for idx := range capConf.CaptureMethods {
				capConf.CaptureMethods[idx].Name = resName
			}
			return nil
		})
}

// Service defines what a Data Manager Service should expose to the users.
type Service interface {
	resource.Resource
	Sync(ctx context.Context, extra map[string]interface{}) error
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("data_manager")

// Subtype is a constant that identifies the data manager service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named datamanager's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// DataCaptureConfigs specify a list of methods to capture on resources.
type DataCaptureConfigs struct {
	CaptureMethods []DataCaptureConfig `json:"capture_methods"`
}

// DataCaptureConfig is used to initialize a collector for a component or remote.
type DataCaptureConfig struct {
	Resource           resource.Resource `json:"-"`
	Name               resource.Name     `json:"name"`
	Method             string            `json:"method"`
	CaptureFrequencyHz float32           `json:"capture_frequency_hz"`
	CaptureQueueSize   int               `json:"capture_queue_size"`
	CaptureBufferSize  int               `json:"capture_buffer_size"`
	AdditionalParams   map[string]string `json:"additional_params"`
	Disabled           bool              `json:"disabled"`
	Tags               []string          `json:"tags"`
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
