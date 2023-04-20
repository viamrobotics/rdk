// Package shell contains a shell service, along with a gRPC server and client
package shell

import (
	"context"

	servicepb "go.viam.com/api/service/shell/v1"

	"go.viam.com/rdk/resource"
)

func init() {
	resource.RegisterSubtype(Subtype, resource.SubtypeRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           servicepb.RegisterShellServiceHandlerFromEndpoint,
		RPCServiceDesc:              &servicepb.ShellService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
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
