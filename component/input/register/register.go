// Package register registers all relevant inputs and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/core/component/camera"
	"go.viam.com/core/component/input"
	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/utils/rpc/dialer"
	rpcserver "go.viam.com/utils/rpc/server"

	// for all inputs
	_ "go.viam.com/core/component/input/fake"
	_ "go.viam.com/core/component/input/gamepad"
	_ "go.viam.com/core/component/input/mux"
	_ "go.viam.com/core/component/input/webgamepad"
)

func init() {
	registry.RegisterResourceSubtype(camera.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return input.WrapWithReconfigurable(r)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpcserver.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.InputControllerService_ServiceDesc,
				camera.NewServer(subtypeSvc),
				pb.RegisterInputControllerServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn dialer.ClientConn, name string, logger golog.Logger) interface{} {
			return camera.NewClientFromConn(conn, name, logger)
		},
	})
}
