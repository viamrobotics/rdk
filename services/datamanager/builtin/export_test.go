// export_test.go adds functionality to the builtin package that we only want to use and expose during testing.
package builtin

import (
	"go.viam.com/rdk/services/datamanager/builtin/sync"
)

// SetSyncerConstructor sets the syncer constructor for the data manager to use when creating its syncer.
func (b *builtIn) SetSyncerConstructor(fn sync.ManagerConstructor) {
	b.sync.SyncerConstructor = fn
}

// SetFileLastModifiedMillis sets the wait time for the syncer to use when initialized/changed in Service.Update.
func (b *builtIn) SetFileLastModifiedMillis(s int) {
	b.sync.FileLastModifiedMillis = s
}
