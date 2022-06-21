// Package internal implements a data manager service definition with additional exported functions for
// the purpose of testing
package internal

import (
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
)

// DMService in the internal package includes additional exported functions relating to the syncing and
// updating processes in the data manager service. These functions are not exported to the user. This resolves
// a circular import caused by the inject package.
type DMService interface {
	Sync(ctx context.Context) error
	Update(ctx context.Context, cfg *config.Config) error
	Close(ctx context.Context) error
	SetUploadFn(fn func(ctx context.Context, path string) error)
	NumCollectors() int
	HasInCollector(componentName string, componentMethodMetadata data.MethodMetadata) bool
}
