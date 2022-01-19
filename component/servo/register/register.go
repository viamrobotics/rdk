// Package register registers all relevant servos and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/servo"

	// registration availability.
	_ "go.viam.com/rdk/component/servo/fake"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(servo.Subtype, registry.ResourceSubtype{
		Reconfigurable: servo.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.ServoService_ServiceDesc,
				servo.NewServer(subtypeSvc),
				componentpb.RegisterServoServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return servo.NewClientFromConn(ctx, conn, name, logger)
		},
	})
}
