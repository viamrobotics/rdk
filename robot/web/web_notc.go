//go:build no_cgo && !android

package web

import (
	"context"

	"go.viam.com/utils/rpc"
)

// stub implementation when gostream not available
func (svc *webService) closeStreamServer() {}

// stub implementation when gostream not available
func (svc *webService) initStreamServer(_ context.Context, _ rpc.Server) error {
	return nil
}

// stub for missing gostream
type options struct{}
