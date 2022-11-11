// Package shell contains a shell service, along with a gRPC server and client
package shell

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	servicepb "go.viam.com/api/service/shell/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.ShellService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterShellServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &servicepb.ShellService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		Reconfigurable: WrapWithReconfigurable,
	})
}

// A Service handles shells for a local robot.
type Service interface {
	Shell(ctx context.Context, extra map[string]interface{}) (input chan<- string, output <-chan Output, retErr error)
}

var (
	_ = Service(&reconfigurableShell{})
	_ = resource.Reconfigurable(&reconfigurableShell{})
	_ = utils.ContextCloser(&reconfigurableShell{})
)

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return rdkutils.NewUnimplementedInterfaceError((Service)(nil), actual)
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

type reconfigurableShell struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableShell) Shell(
	ctx context.Context,
	extra map[string]interface{},
) (input chan<- string, output <-chan Output, retErr error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Shell(ctx, extra)
}

func (svc *reconfigurableShell) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return utils.TryClose(ctx, svc.actual)
}

// Reconfigure replaces the old shell service with a new shell.
func (svc *reconfigurableShell) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableShell)
	if !ok {
		return rdkutils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := utils.TryClose(ctx, svc.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a shell service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(s)
	}

	if reconfigurable, ok := s.(*reconfigurableShell); ok {
		return reconfigurable, nil
	}

	return &reconfigurableShell{actual: svc}, nil
}
