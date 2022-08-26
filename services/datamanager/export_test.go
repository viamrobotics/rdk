// export_test.go adds functionality to the datamanager that we only want to use and expose during testing.
package datamanager

import (
	"fmt"

	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/services/datamanager/model"
	"go.viam.com/utils/rpc"
)

// SetSyncerConstructor sets the syncer constructor for the data manager to use when creating its syncer.
func (svc *dataManagerService) SetSyncerConstructor(fn datasync.ManagerConstructor) {
	svc.syncerConstructor = fn
}

func (svc *modelManagerService) SetModelrConstructor(fn model.ManagerConstructor) {
	svc.modelrConstructor = fn
}

// SetSyncerConstructor sets the syncer constructor for the data manager to use when creating its syncer.
func (svc *dataManagerService) SetSyncer(s datasync.Manager) {
	svc.syncer = s
}

// SetWaitAfterLastModifiedSecs sets the wait time for the syncer to use when initialized/changed in Service.Update.
func (svc *dataManagerService) SetWaitAfterLastModifiedSecs(s int) {
	svc.waitAfterLastModifiedSecs = s
}

func (svc *modelManagerService) SetWaitAfterLastModifiedSecs(s int) {
	svc.waitAfterLastModifiedSecs = s
}

func (svc *modelManagerService) SetClientConn(c rpc.ClientConn) {
	// fmt.Println("export_test.go/SetClientConn() with value c: ", c)
	svc.clientConn = &c
	fmt.Println("svc.clientConn: ", svc.clientConn)
}

// Make getServiceConfig global for tests.
var GetServiceConfig = getServiceConfig

// Make getDurationFromHz global for tests.
var GetDurationFromHz = getDurationFromHz
