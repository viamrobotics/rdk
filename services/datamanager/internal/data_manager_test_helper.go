// Package internal implements a data manager service definition with additional exported functions for
// the purpose of testing
package internal

import (
	"context"

	urpc "go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/services/datamanager/model"
)

// DMService in the internal package includes additional exported functions relating to the syncing and
// updating processes in the data manager service. These functions are not exported to the user. This resolves
// a circular import caused by the inject package.
type DMService interface {
	Sync(ctx context.Context) error
	Update(ctx context.Context, cfg *config.Config) error
	Close(ctx context.Context) error
	SetSyncerConstructor(fn datasync.ManagerConstructor)
	SetSyncer(s datasync.Manager)
	SetWaitAfterLastModifiedSecs(s int)
	SetModelrConstructor(fn model.ManagerConstructor)
	SetClientConn(c urpc.ClientConn)
}
