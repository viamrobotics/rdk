package shell

import (
	"errors"
	"io"

	"go.uber.org/multierr"
	goutils "go.viam.com/utils"

	pb "go.viam.com/rdk/proto/api/service/shell/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the contract from shell.proto.
type subtypeServer struct {
	pb.UnimplementedShellServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a framesystem gRPC service server.
func NewServer(s subtype.Service) pb.ShellServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service(serviceName string) (Service, error) {
	resource := server.subtypeSvc.Resource(serviceName)
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Named(serviceName))
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("shell.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) Shell(srv pb.ShellService_ShellServer) (retErr error) {
	firstMsg := true
	req, err := srv.Recv()
	errTemp := err
	svc, err := server.service(req.Name)
	if err != nil {
		return err
	}
	input, output, err := svc.Shell(srv.Context())
	if err != nil {
		return err
	}

	inDone := make(chan error)
	outDone := make(chan struct{})
	defer func() {
		retErr = multierr.Combine(retErr, <-inDone)
	}()

	goutils.PanicCapturingGo(func() {
		defer close(inDone)

		for {
			if firstMsg {
				firstMsg = false
				err = errTemp
			} else {
				req, err = srv.Recv()
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					close(input)
					break
				}
				inDone <- err
				return
			}

			select {
			case input <- req.DataIn:
			case <-outDone:
				close(input)
				return
			case <-srv.Context().Done():
				inDone <- srv.Context().Err()
				return
			}
		}
	})

	defer close(outDone)
	for {
		select {
		case out, ok := <-output:
			if ok {
				if err := srv.Send(&pb.ShellResponse{
					DataOut: out.Output,
					DataErr: out.Error,
					Eof:     out.EOF,
				}); err != nil {
					return srv.Context().Err()
				}
				if out.EOF {
					return nil
				}
			} else {
				return srv.Send(&pb.ShellResponse{
					Eof: true,
				})
			}
		case <-srv.Context().Done():
			return srv.Context().Err()
		}
	}
}
