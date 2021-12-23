// Package register registers all relevant IMUs and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/imu"
	_ "go.viam.com/rdk/component/imu/fake" // for imu
	_ "go.viam.com/rdk/component/imu/wit"  // for imu
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(imu.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return imu.WrapWithReconfigurable(r)
		},
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
