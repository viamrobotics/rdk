// Package shell contains a shell service, along with a gRPC server and client
package shell

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/creack/pty"
	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/shell/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
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
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(logger)
		},
	},
	)

	resource.AddDefaultService(Name)
}

// A Service handles shells for a local robot.
type Service interface {
	Shell(ctx context.Context) (input chan<- string, output <-chan Output, retErr error)
}

var (
	_ = Service(&reconfigurableShell{})
	_ = resource.Reconfigurable(&reconfigurableShell{})
)

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

// Name is the ShellService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// New returns a new shell service for the given robot.
func New(logger golog.Logger) (Service, error) {
	return &shellService{logger: logger}, nil
}

type shellService struct {
	logger                  golog.Logger
	activeBackgroundWorkers sync.WaitGroup
}

func (svc *shellService) Shell(ctx context.Context) (chan<- string, <-chan Output, error) {
	if runtime.GOOS == "windows" {
		return nil, nil, errors.New("shell not supported on windows yet; sorry")
	}
	defaultShellPath, ok := os.LookupEnv("SHELL")
	if !ok {
		defaultShellPath = "/bin/sh"
	}

	ctxCancel, cancel := context.WithCancel(ctx)
	//nolint:gosec
	cmd := exec.CommandContext(ctxCancel, defaultShellPath, "-i")
	f, err := pty.Start(cmd)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	svc.activeBackgroundWorkers.Add(2)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		defer cancel()
		<-ctx.Done()
	})
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()
		defer cancel()
		if err := cmd.Wait(); err != nil {
			svc.logger.Debugw("error waiting for cmd", "error", err)
		}
		if err := f.Close(); err != nil {
			svc.logger.Debugw("error closing pty", "error", err)
		}
	})

	input := make(chan string)
	output := make(chan Output)

	utils.PanicCapturingGo(func() {
		defer close(output)
		var data [64]byte
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := f.Read(data[:])
			if err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
					svc.logger.Errorw("error reading output", "error", err)
				}
				select {
				case <-ctx.Done():
					return
				case output <- Output{EOF: true}:
				}
				return
			}
			if n == 0 {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case output <- Output{Output: string(data[:n])}:
			}
		}
	})

	utils.PanicCapturingGo(func() {
		for {
			select {
			case inputData, ok := <-input:
				if ok {
					if _, err := f.Write([]byte(inputData)); err != nil {
						svc.logger.Errorw("error writing data", "error", err)
						return
					}
				} else {
					if _, err := f.Write([]byte{4}); err != nil {
						svc.logger.Errorw("error writing EOT", "error", err)
						return
					}
					return
				}
			case <-ctx.Done():
				if _, err := f.Write([]byte{4}); err != nil {
					svc.logger.Errorw("error writing EOT", "error", err)
					return
				}
			}
		}
	})

	return input, output, nil
}

func (svc *shellService) Close() {
	svc.activeBackgroundWorkers.Wait()
}

type reconfigurableShell struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableShell) Shell(ctx context.Context) (input chan<- string, output <-chan Output, retErr error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Shell(ctx)
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
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a shell service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, rdkutils.NewUnimplementedInterfaceError("shell.Service", s)
	}

	if reconfigurable, ok := s.(*reconfigurableShell); ok {
		return reconfigurable, nil
	}

	return &reconfigurableShell{actual: svc}, nil
}
