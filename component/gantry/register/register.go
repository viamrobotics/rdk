// Package register registers all relevant gantries and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/gantry"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"

	_ "go.viam.com/rdk/component/gantry/fake"   // for gantry
	_ "go.viam.com/rdk/component/gantry/simple" // for gantry
)

func init() {
	registry.RegisterResourceSubtype(gantry.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return gantry.WrapWithReconfigurable(r)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.GantryService_ServiceDesc,
				gantry.NewServer(subtypeSvc),
				componentpb.RegisterGantryServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return gantry.NewClientFromConn(conn, name, logger)
		},
	})
}
