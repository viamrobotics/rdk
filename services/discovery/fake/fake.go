// Package fake implements a fake discovery service.
package fake

import (
	"context"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
)

func init() {
	resource.RegisterService(
		discovery.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[discovery.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (discovery.Service, error) {
			return newDiscovery(conf.ResourceName(), logger), nil
		}})
}

func newDiscovery(name resource.Name, logger logging.Logger) discovery.Service {
	cfg1 := createFakeConfig("fake1")
	cfg2 := createFakeConfig("fake2")
	return &Discovery{Named: name.AsNamed(), logger: logger, cfgs: []resource.Config{cfg1, cfg2}}
}

func (dis *Discovery) DiscoverResources(context.Context, map[string]any) ([]resource.Config, error) {

	return nil, nil
}

// Discovery is a fake Discovery service that returns
type Discovery struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	logger logging.Logger
	cfgs   []resource.Config
}

// DoCommand echos input back to the caller.
func (fg *Discovery) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func createFakeConfig(name string) resource.Config {
	// // using the camera's Config struct in case a breaking change occurs
	// attributes := viamrtsp.Config{Address: address}
	// var result map[string]interface{}

	// // marshal to bytes
	// jsonBytes, err := json.Marshal(attributes)
	// if err != nil {
	// 	return resource.Config{}, err
	// }

	// // convert to map to be used as attributes in resource.Config
	// err = json.Unmarshal(jsonBytes, &result)
	// if err != nil {
	// 	return resource.Config{}, err
	// }
	return resource.Config{Name: name, API: movementsensor.API, Model: resource.DefaultModelFamily.WithModel("fake"), Attributes: nil}
}
