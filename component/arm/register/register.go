// Package register registers all relevant arms and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/arm"

	// register eva.
	_ "go.viam.com/rdk/component/arm/eva"

	// register fake arm.
	_ "go.viam.com/rdk/component/arm/fake"

	// register UR.
	_ "go.viam.com/rdk/component/arm/universalrobots"

	// register varm.
	_ "go.viam.com/rdk/component/arm/varm"

	// register vx300s.
	_ "go.viam.com/rdk/component/arm/vx300s"

	// register wx250s.
	_ "go.viam.com/rdk/component/arm/wx250s"

	// register xArm.
	_ "go.viam.com/rdk/component/arm/xarm"

	// register yahboom.
	_ "go.viam.com/rdk/component/arm/yahboom"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(arm.Subtype, registry.ResourceSubtype{
		Reconfigurable: arm.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.ArmService_ServiceDesc,
				arm.NewServer(subtypeSvc),
				componentpb.RegisterArmServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return arm.NewClientFromConn(conn, name, logger)
		},
	})
}
