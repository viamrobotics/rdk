// export_test.go adds functionality to dataManagerService that we only use during testing.
package datamanager

import (
	"context"

	"go.viam.com/rdk/config"
)

func (svc *dataManagerService) SetUploadFn(fn func(ctx context.Context, path string) error) {
	svc.uploadFn = fn
}

// Get the config associated with the data manager service.
func GetDataManagerServiceConfig(cfg *config.Config) (*config.Service, bool) {
	for _, c := range cfg.Services {
		// Compare service type and name.
		if c.ResourceName() == Name {
			return &c, true
		}
	}
	return &config.Service{}, false
}
