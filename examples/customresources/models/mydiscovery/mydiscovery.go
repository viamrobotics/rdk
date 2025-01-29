// Package mydiscovery implements a discovery that returns some fake components.
package mydiscovery

import (
	"context"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
)

// Model is the full model definition.
var Model = resource.NewModel("acme", "demo", "mydiscovery")

func init() {
	resource.RegisterService(
		discovery.API,
		Model,
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
	cfg1 := createFakeConfig("fake1", movementsensor.API)
	cfg2 := createFakeConfig("fake2", camera.API)
	return &Discovery{Named: name.AsNamed(), logger: logger, cfgs: []resource.Config{cfg1, cfg2}}
}

// DiscoverResources returns the discovered resources.
func (dis *Discovery) DiscoverResources(context.Context, map[string]any) ([]resource.Config, error) {
	return dis.cfgs, nil
}

// Discovery is a fake Discovery service.
type Discovery struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	logger logging.Logger
	cfgs   []resource.Config
}

// DoCommand echos input back to the caller.
func (dis *Discovery) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

// createFakeConfig creates a fake component with the defined name, api, and attributes.
func createFakeConfig(name string, api resource.API) resource.Config {
	return resource.Config{Name: name, API: api, Model: resource.DefaultModelFamily.WithModel("fake")}
}
