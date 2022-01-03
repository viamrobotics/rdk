// Package register registers all relevant inputs and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/utils/rpc"

	// for all inputs.
	"go.viam.com/rdk/component/input"
	_ "go.viam.com/rdk/component/input/fake"
	_ "go.viam.com/rdk/component/input/gamepad"
	_ "go.viam.com/rdk/component/input/mux"
	_ "go.viam.com/rdk/component/input/webgamepad"
)

func init() {
	registry.RegisterResourceSubtype(input.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return input.WrapWithReconfigurable(r)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.InputControllerService_ServiceDesc,
				input.NewServer(subtypeSvc),
				pb.RegisterInputControllerServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return input.NewClientFromConn(conn, name, logger)
		},
	})
}
