// Package register registers all relevant arms and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc/dialer"
	rpcserver "go.viam.com/utils/rpc/server"

	"go.viam.com/core/component/arm"
	componentpb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
)

func init() {
	registry.RegisterResourceSubtype(arm.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return arm.WrapWithReconfigurable(r)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpcserver.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.ArmService_ServiceDesc,
				arm.NewServer(subtypeSvc),
				componentpb.RegisterArmServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn dialer.ClientConn, name string, logger golog.Logger) interface{} {
			return arm.NewClientFromConn(conn, name, logger)
		},
	})

}
