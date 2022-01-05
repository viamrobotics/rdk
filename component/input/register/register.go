// Package register registers all relevant inputs and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/input"

	// for all inputs.
	_ "go.viam.com/rdk/component/input/fake"
	_ "go.viam.com/rdk/component/input/gamepad"
	_ "go.viam.com/rdk/component/input/mux"
	_ "go.viam.com/rdk/component/input/webgamepad"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(input.Subtype, registry.ResourceSubtype{
		Reconfigurable: input.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.InputControllerService_ServiceDesc,
				input.NewServer(subtypeSvc),
				pb.RegisterInputControllerServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return input.NewClientFromConn(ctx, conn, name, logger)
		},
	})
}
