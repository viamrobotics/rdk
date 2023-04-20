// Package shell contains a shell service, along with a gRPC server and client
package shell

import (
	"context"

	"github.com/edaniels/golog"
	servicepb "go.viam.com/api/service/shell/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype[Service]{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeColl resource.SubtypeCollection[Service]) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.ShellService_ServiceDesc,
				NewServer(subtypeColl),
				servicepb.RegisterShellServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &servicepb.ShellService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (Service, error) {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// A Service handles shells for a local robot.
type Service interface {
	resource.Resource
	Shell(ctx context.Context, extra map[string]interface{}) (input chan<- string, output <-chan Output, retErr error)
}

// Output reflects an instance of shell output on either stdout or stderr.
type Output struct {
	Output string // reflects stdout
	Error  string // reflects stderr
	EOF    bool
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("shell")

// Subtype is a constant that identifies the shell service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}
