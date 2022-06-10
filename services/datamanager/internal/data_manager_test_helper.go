// Package internal implements a data manager service definition with additional exported functions for
// the purpose of testing
package internal

import (
	"context"

	"go.viam.com/rdk/config"
)

// Service in the internal package includes additional exported functions relating to the syncing and
// updating processes in the data manager service. These functions are not exported to the user. This resolves
// a circular import caused by the inject package.
type Service interface {
	Sync(ctx context.Context) error
	Update(ctx context.Context, cfg *config.Config) error
	QueueCapturedData(cancelCtx context.Context, intervalMins int)
}
