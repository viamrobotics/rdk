// Package inject provides an mock cloud connection service that can be used for testing.
package inject

import (
	"context"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/resource"
)

// CloudConnectionService stores the functions and variables to create a mock cloud connection service.
type CloudConnectionService struct {
	resource.Named
	resource.AlwaysRebuild
	Conn rpc.ClientConn
}

// AcquireConnection returns a connection to the rpc server stored in the mockCloudConnectionService object.
func (noop *CloudConnectionService) AcquireConnection(ctx context.Context) (string, rpc.ClientConn, error) {
	return "", noop.Conn, nil
}

// Close is used by the mockCloudConnectionService to complete the cloud connection service interface.
func (noop *CloudConnectionService) Close(ctx context.Context) error {
	return nil
}
