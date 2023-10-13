package shell

import (
	"context"
	"errors"
	"io"

	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/shell/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the contract from shell.proto.
type serviceServer struct {
	pb.UnimplementedShellServiceServer
	coll resource.APIResourceCollection[Service]
}

// NewRPCServiceServer constructs a framesystem gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Service]) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) Shell(srv pb.ShellService_ShellServer) (retErr error) {
	firstMsg := true
	req, err := srv.Recv()
	errTemp := err
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return err
	}
	input, output, err := svc.Shell(srv.Context(), req.Extra.AsMap())
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

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
