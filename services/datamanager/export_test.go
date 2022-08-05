// export_test.go adds functionality to the datamanager that we only want to use and expose during testing.
package datamanager

import (
	"go.viam.com/rdk/services/datamanager/datasync"
)

// SetUploadFunc sets the upload function for the syncer to use when initialized/changed in Service.Update.
func (svc *dataManagerService) SetUploadFunc(fn datasync.UploadFunc) {
	svc.uploadFunc = fn
}

// SetSyncerConstructor sets the syncer constructor for the data manager to use when creating its syncer.
func (svc *dataManagerService) SetSyncerConstructor(fn datasync.ManagerConstructor) {
	svc.syncerConstructor = fn
}

// SetWaitAfterLastModifiedSecs sets the wait time for the syncer to use when initialized/changed in Service.Update.
func (svc *dataManagerService) SetWaitAfterLastModifiedSecs(s int) {
	svc.waitAfterLastModifiedSecs = s
}

// Make getServiceConfig global for tests.
var GetServiceConfig = getServiceConfig

// Make getDurationFromHz global for tests.
var GetDurationFromHz = getDurationFromHz
