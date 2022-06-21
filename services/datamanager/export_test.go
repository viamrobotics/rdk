// export_test.go adds functionality to the datamanager that we only want to use and expose during testing.
package datamanager

import (
	"context"

	"go.viam.com/rdk/data"
)

// SetUploadFn sets the upload function for the syncer to use when initialized/changed in Service.Update.
func (svc *dataManagerService) SetUploadFn(fn func(ctx context.Context, path string) error) {
	svc.uploadFn = fn
}

// NumCollectors returns the number of collectors the data manager service currently has.
func (svc *dataManagerService) NumCollectors() int {
	return len(svc.collectors)
}

func (svc *dataManagerService) HasInCollector(componentName string, MethodMetadata data.MethodMetadata) bool {
	compMethodMetadata := componentMethodMetadata{
		componentName, MethodMetadata,
	}
	_, present := svc.collectors[compMethodMetadata]
	return present
}

// Make getServiceConfig global for tests.
var GetServiceConfig = getServiceConfig
