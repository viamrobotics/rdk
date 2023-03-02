// Package thingamabobapi defines an empty API to demonstrate custom validation logic.
package thingamabobapi

import (
	"context"

	"github.com/edaniels/golog"
	testecho "go.viam.com/api/component/testecho/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
)

var Subtype = resource.NewSubtype(
	resource.Namespace("acme"),
	resource.ResourceTypeComponent,
	resource.SubtypeName("thingamabob"),
)

type Thingamabob interface{}

func init() {
	// Register a simple echo service as a placeholder API.
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&echopb.EchoService_ServiceDesc,
				&echoserver.Server{},
				echopb.RegisterEchoServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &echopb.EchoService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return testecho.NewTestEchoServiceClient(conn)
		},
	})

}
