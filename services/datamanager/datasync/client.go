package datasync

import (
	"context"
	"go.uber.org/zap"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/utils/rpc"
)

// NewSvcClientFromConn constructs a new serviceClient using the passed in connection.
func NewClient(conn rpc.ClientConn) v1.DataSyncServiceClient {
	return v1.NewDataSyncServiceClient(conn)
}

func NewConnection(logger *zap.SugaredLogger, address string, rpcOpts []rpc.DialOption) (rpc.ClientConn, error) {
	ctx := context.Background()
	//tlsConfig := config.NewTLSConfig(cfg).Config
	//cloudConfig := cfg.Cloud
	//rpcOpts := []rpc.DialOption{
	//	rpc.WithTLSConfig(tlsConfig),
	//	rpc.WithEntityCredentials(
	//		cloudConfig.ID,
	//		rpc.Credentials{
	//			Type:    rdkutils.CredentialsTypeRobotLocationSecret,
	//			Payload: cloudConfig.LocationSecret,
	//		}),
	//}
	//appURL := "app.viam.com:443" // TODO: Find way to not hardcode this. Maybe look in grpc/dial.go?

	conn, err := rpc.DialDirectGRPC(
		ctx,
		address,
		logger,
		rpcOpts...,
	)
	return conn, err
}
