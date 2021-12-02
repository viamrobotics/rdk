// register registers all relevant gantries and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/core/component/gantry"
	componentpb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/utils/rpc/dialer"
	rpcserver "go.viam.com/utils/rpc/server"

	_ "go.viam.com/core/component/gantry/fake"
	_ "go.viam.com/core/component/gantry/simple"
)

func init() {
	registry.RegisterResourceSubtype(gantry.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return gantry.WrapWithReconfigurable(r)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpcserver.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.GantryService_ServiceDesc,
				gantry.NewServer(subtypeSvc),
				componentpb.RegisterGantryServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn dialer.ClientConn, name string, logger golog.Logger) interface{} {
			return gantry.NewClientFromConn(conn, name, logger)
		},
	})
}
