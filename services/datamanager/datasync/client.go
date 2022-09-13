package datasync

import (
	"context"

	"go.uber.org/zap"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/utils/rpc"
)

// NewClient constructs a new v1.DataSyncServiceClient using the passed in connection.
func NewClient(conn rpc.ClientConn) v1.DataSyncServiceClient {
	return v1.NewDataSyncServiceClient(conn)
}

// NewConnection builds a connection to the passed address with the passed rpcOpts.
func NewConnection(logger *zap.SugaredLogger, address string, rpcOpts []rpc.DialOption) (rpc.ClientConn, error) {
	ctx := context.Background()
	conn, err := rpc.DialDirectGRPC(
		ctx,
		address,
		logger,
		rpcOpts...,
	)
	return conn, err
}
