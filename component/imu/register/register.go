// Package register registers all relevant IMUs and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc/dialer"
	rpcserver "go.viam.com/utils/rpc/server"

	"go.viam.com/core/component/imu"
	_ "go.viam.com/core/component/imu/fake" // for imu
	_ "go.viam.com/core/component/imu/wit"  // for imu
	componentpb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
)

func init() {
	registry.RegisterResourceSubtype(imu.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return imu.WrapWithReconfigurable(r)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpcserver.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.IMUService_ServiceDesc,
				imu.NewServer(subtypeSvc),
				componentpb.RegisterIMUServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn dialer.ClientConn, name string, logger golog.Logger) interface{} {
			return imu.NewClientFromConn(conn, name, logger)
		},
	})
}
