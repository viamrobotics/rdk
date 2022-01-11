// Package register registers all relevant ForceMatrix's and
// also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/forcematrix"

	// register various implementations of ForceMatrix.
	_ "go.viam.com/rdk/component/forcematrix/fake"
	_ "go.viam.com/rdk/component/forcematrix/vforcematrixtraditional"
	_ "go.viam.com/rdk/component/forcematrix/vforcematrixwithmux"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(forcematrix.Subtype, registry.ResourceSubtype{
		Reconfigurable: forcematrix.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.ForceMatrixService_ServiceDesc,
				forcematrix.NewServer(subtypeSvc),
				componentpb.RegisterForceMatrixServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return forcematrix.NewClientFromConn(ctx, conn, name, logger)
		},
	})
}
