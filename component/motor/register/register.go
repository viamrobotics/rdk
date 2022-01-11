// Package register registers all relevant motors and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/motor"

	// registration availability.
	_ "go.viam.com/rdk/component/motor/fake"

	// pi motor.
	_ "go.viam.com/rdk/component/motor/gpio"

	// pi stepper motor.
	_ "go.viam.com/rdk/component/motor/gpiostepper"

	// tmc stepper motor.
	_ "go.viam.com/rdk/component/motor/tmcstepper"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/subtype"
)

func init() {
	registry.RegisterResourceSubtype(motor.Subtype, registry.ResourceSubtype{
		Reconfigurable: motor.WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.MotorService_ServiceDesc,
				motor.NewServer(subtypeSvc),
				componentpb.RegisterMotorServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return motor.NewClientFromConn(ctx, conn, name, logger)
		},
	})
}
