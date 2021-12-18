// Package register registers all relevant servos and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/utils/rpc"

	"go.viam.com/core/component/servo"
	componentpb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"

	// all servo implementations should be imported here for
	// registration availability
	_ "go.viam.com/core/component/servo/fake" // fake servo implementations
)

func init() {
	registry.RegisterResourceSubtype(servo.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(resource interface{}) (resource.Reconfigurable, error) {
			return servo.WrapWithReconfigurable(resource)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.ServoService_ServiceDesc,
				servo.NewServer(subtypeSvc),
				componentpb.RegisterServoServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return servo.NewClientFromConn(conn, name, logger)
		},
	})
}
