// Package inject provides an mock cloud connection service that can be used for testing.
package inject

import (
	"context"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/resource"
)

// CloudConnectionService is an implementation of the cloud.ConnectionService interface used for testing.
type CloudConnectionService struct {
	resource.Named
	resource.AlwaysRebuild
	Conn                 rpc.ClientConn
	AcquireConnectionErr error
}

// AcquireConnection returns a connection to the rpc server stored in the cloud connection service object.
func (cloudConnService *CloudConnectionService) AcquireConnection(ctx context.Context) (string, rpc.ClientConn, error) {
	if cloudConnService.AcquireConnectionErr != nil {
		return "", nil, cloudConnService.AcquireConnectionErr
	}
	return "hello", cloudConnService.Conn, nil
}

// AcquireConnectionAPIKey returns a connection to the rpc server stored in the cloud connection service object.
func (cloudConnService *CloudConnectionService) AcquireConnectionAPIKey(ctx context.Context,
	apiKey, apiKeyID string,
) (string, rpc.ClientConn, error) {
	if cloudConnService.AcquireConnectionErr != nil {
		return "", nil, cloudConnService.AcquireConnectionErr
	}
	return "hello", cloudConnService.Conn, nil
}

// Close is used by the CloudConnectionService to complete the cloud.ConnectionService interface.
func (cloudConnService *CloudConnectionService) Close(ctx context.Context) error {
	return nil
}
