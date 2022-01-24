// Package register registers all relevant Sensors and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/sensor"

	// for Sensors.
	_ "go.viam.com/rdk/component/sensor/fake"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(sensor.Subtype, registry.ResourceSubtype{
		Reconfigurable: sensor.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.SensorService_ServiceDesc,
				sensor.NewServer(subtypeSvc),
				pb.RegisterSensorServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return sensor.NewClientFromConn(ctx, conn, name, logger)
		},
	})
}
