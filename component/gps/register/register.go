// Package register registers all relevant GPSs and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/gps"

	// for GPSs.
	_ "go.viam.com/rdk/component/gps/fake"
	_ "go.viam.com/rdk/component/gps/merge"
	_ "go.viam.com/rdk/component/gps/nmea"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(gps.Subtype, registry.ResourceSubtype{
		Reconfigurable: gps.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.GPSService_ServiceDesc,
				gps.NewServer(subtypeSvc),
				pb.RegisterGPSServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return gps.NewClientFromConn(ctx, conn, name, logger)
		},
	})
}
