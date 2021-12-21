// Package register registers all relevant grippers and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/utils/rpc"

	"go.viam.com/core/component/gripper"
	componentpb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"

	_ "go.viam.com/core/component/gripper/fake"         // for a gripper
	_ "go.viam.com/core/component/gripper/robotiq"      // for a gripper
	_ "go.viam.com/core/component/gripper/softrobotics" // for a gripper
	_ "go.viam.com/core/component/gripper/vgripper/v1"  // for a gripper with a single force sensor cell
	_ "go.viam.com/core/component/gripper/vgripper/v2"  // for a gripper with a force matrix
	_ "go.viam.com/core/component/gripper/vx300s"       // for a gripper
	_ "go.viam.com/core/component/gripper/wx250s"       // for a gripper
	_ "go.viam.com/core/component/gripper/yahboom"      // for a gripper
)

func init() {
	registry.RegisterResourceSubtype(gripper.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return gripper.WrapWithReconfigurable(r)
		},
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
