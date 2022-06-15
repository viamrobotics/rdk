// Package internal implements a data manager service definition with additional exported functions for
// the purpose of testing
package internal

import (
	"context"

	"go.viam.com/rdk/config"
)

// DMService in the internal package includes additional exported functions relating to the syncing and
// updating processes in the data manager service. These functions are not exported to the user. This resolves
// a circular import caused by the inject package.
type DMService interface {
	Sync(ctx context.Context) error
	Update(ctx context.Context, cfg *config.Config) error
	QueueCapturedData(cancelCtx context.Context, intervalMins int)
	Close(ctx context.Context) error
	SetUploadFn(fn func(ctx context.Context, path string) error)
	StartSyncer()
}

// SyncService in the internal package includes additional exported functions relating to the syncer.
// These functions are not exported to the user. This resolves circular import caused by the inject package,
// as the syncer lives in the same package as the datamanager, and we have a mock datamanager in inject.
type SyncService interface {
	Start()
	Enqueue(filesToQueue []string) error
	Upload()
	Close()
	SetUploadFn(fn func(ctx context.Context, path string) error)
}
