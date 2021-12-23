// Package register registers all relevant motors and also subtype specific functions
package register

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/motor"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"

	// all motor implementations should be imported here for
	// registration availability
	_ "go.viam.com/rdk/component/motor/fake"        // fake motor
	_ "go.viam.com/rdk/component/motor/gpio"        // pi motor
	_ "go.viam.com/rdk/component/motor/gpiostepper" // pi stepper motor
	_ "go.viam.com/rdk/component/motor/tmcstepper"  // tmc stepper motor
)

func init() {
	registry.RegisterResourceSubtype(motor.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return motor.WrapWithReconfigurable(r)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&componentpb.MotorService_ServiceDesc,
				motor.NewServer(subtypeSvc),
				componentpb.RegisterMotorServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return motor.NewClientFromConn(conn, name, logger)
		},
	})
}
