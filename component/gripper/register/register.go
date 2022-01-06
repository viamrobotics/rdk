// Package register registers all relevant grippers and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/gripper"

	// for grippers.
	_ "go.viam.com/rdk/component/gripper/fake"
	_ "go.viam.com/rdk/component/gripper/robotiq"
	_ "go.viam.com/rdk/component/gripper/softrobotics"
	_ "go.viam.com/rdk/component/gripper/vgripper/v1"
	_ "go.viam.com/rdk/component/gripper/vgripper/v2"
	_ "go.viam.com/rdk/component/gripper/vx300s"
	_ "go.viam.com/rdk/component/gripper/wx250s"
	_ "go.viam.com/rdk/component/gripper/yahboom"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(gripper.Subtype, registry.ResourceSubtype{
		Reconfigurable: gripper.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.GripperService_ServiceDesc,
				gripper.NewServer(subtypeSvc),
				componentpb.RegisterGripperServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return gripper.NewClientFromConn(conn, name, logger)
		},
	})
}
