// Package register registers all relevant IMUs and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/imu"

	// for imu.
	_ "go.viam.com/rdk/component/imu/fake"

	// for imu.
	_ "go.viam.com/rdk/component/imu/wit"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(imu.Subtype, registry.ResourceSubtype{
		Reconfigurable: imu.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.IMUService_ServiceDesc,
				imu.NewServer(subtypeSvc),
				componentpb.RegisterIMUServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return imu.NewClientFromConn(conn, name, logger)
		},
	})
}
