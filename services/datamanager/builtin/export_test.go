// export_test.go adds functionality to the builtin package that we only want to use and expose during testing.
package builtin

import (
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/services/datamanager/model"
)

// SetSyncerConstructor sets the syncer constructor for the data manager to use when creating its syncer.
func (svc *builtIn) SetSyncerConstructor(fn datasync.ManagerConstructor) {
	svc.syncerConstructor = fn
}

// SetModelrConstructor sets the modelManager constructor for the data manager to use when creating its modelManager.
func (svc *builtIn) SetModelManagerConstructor(fn model.ManagerConstructor) {
	svc.modelManagerConstructor = fn
}

// SetWaitAfterLastModifiedSecs sets the wait time for the syncer to use when initialized/changed in Service.Update.
func (svc *builtIn) SetWaitAfterLastModifiedMillis(s int) {
	svc.waitAfterLastModifiedMillis = s
}

// Make getServiceConfig global for tests.
var GetServiceConfig = getServiceConfig

// Make getDurationFromHz global for tests.
var GetDurationFromHz = getDurationFromHz
