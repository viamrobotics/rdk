// export_test.go adds functionality to the datamanager that we only want to use and expose during testing.
package datamanager

import (
	"context"

	v1 "go.viam.com/api/proto/viam/datasync/v1"
)

// SetUploadFn sets the upload function for the syncer to use when initialized/changed in Service.Update.
func (svc *dataManagerService) SetUploadFn(fn func(ctx context.Context, pt ProgressTracker, client v1.DataSyncService_UploadClient,
	path string, partID string) error,
) {
	svc.uploadFunc = fn
}

// SetWaitAfterLastModifiedSecs sets the wait time for the syncer to use when initialized/changed in Service.Update.
func (svc *dataManagerService) SetWaitAfterLastModifiedSecs(s int) {
	svc.waitAfterLastModifiedSecs = s
}

// Make getServiceConfig global for tests.
var GetServiceConfig = getServiceConfig
