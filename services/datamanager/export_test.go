// export_test.go adds functionality to the datamanager that we only want to use and expose during testing.
package datamanager

import (
	"context"

	v1 "go.viam.com/api/proto/viam/datasync/v1"
)

// SetUploadFn sets the upload function for the syncer to use when initialized/changed in Service.Update.
func (svc *dataManagerService) SetUploadFn(fn func(ctx context.Context, client v1.DataSyncService_UploadClient,
	path string, partID string) error,
) {
	svc.uploadFunc = fn
}

// SetWaitAfterLastModify sets the wait time for the syncer to use when initialized/changed in Service.Update.
func (svc *dataManagerService) SetWaitAfterLastModify(s int) {
	svc.waitAfterLastModify = s
}

// Make getServiceConfig global for tests.
var GetServiceConfig = getServiceConfig
