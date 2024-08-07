package shell

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"syscall"

	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/shell/v1"
	"go.viam.com/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	input, oobInput, output, err := svc.Shell(srv.Context(), req.Extra.AsMap())
	if err != nil {
		return err
	}

	inDone := make(chan error)
	outDone := make(chan struct{})
	defer func() {
		retErr = multierr.Combine(retErr, <-inDone)
	}()

	utils.PanicCapturingGo(func() {
		defer close(inDone)

		for {
			if firstMsg {
				firstMsg = false
				err = errTemp
				req.Extra = nil
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

			if req.Extra != nil {
				ext := req.Extra.AsMap()
				if len(ext) != 0 {
					select {
					case oobInput <- ext:
					case <-outDone:
						close(input)
						return
					case <-srv.Context().Done():
						inDone <- srv.Context().Err()
						return
					}
				}
			}
			if len(req.DataIn) == 0 {
				continue
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

// CopyFilesToMachine is the server side RPC implementation of copying files to a machine.
// It'll receive the initial metadata of the request, call the underlying service's CopyFilesToMachine
// method, and forward files to its FileCopier via an RPC based FileReadCopier.
func (server *serviceServer) CopyFilesToMachine(srv pb.ShellService_CopyFilesToMachineServer) error {
	mdReq, err := srv.Recv()
	if err != nil {
		return err
	}
	md, ok := mdReq.Request.(*pb.CopyFilesToMachineRequest_Metadata)
	if !ok {
		return errors.New("expected copy request metadata")
	}
	svc, err := server.coll.Resource(md.Metadata.Name)
	if err != nil {
		return err
	}
	fileCopier, err := svc.CopyFilesToMachine(
		srv.Context(),
		CopyFilesSourceTypeFromProto(md.Metadata.SourceType),
		md.Metadata.Destination,
		md.Metadata.Preserve,
		md.Metadata.Extra.AsMap())
	if err != nil {
		var pathErr *fs.PathError
		var errno syscall.Errno
		if errors.As(err, &pathErr) && errors.As(pathErr.Err, &errno) {
			// we use an error code here so CLI can detect this case and give instructions
			return status.New(codes.PermissionDenied, err.Error()).Err()
		}
		return err
	}
	defer func() {
		utils.UncheckedError(fileCopier.Close(srv.Context()))
	}()

	// create a FileCopyReader that has a Read/Copy pipeline of:
	// CopyFilesToMachineClient->ShellRPCFileReadCopier->copier
	// ShellRPCFileReadCopier does the heavy lifting for us by handling fragmentation
	// and ordering of files coming in.
	reader := newShellRPCFileReadCopier(shellRPCCopyReaderTo{srv}, fileCopier)
	defer func() {
		utils.UncheckedError(reader.Close(srv.Context()))
	}()
	return reader.ReadAll(srv.Context())
}

// CopyFilesFromMachine is the server side RPC implementation of copying files from a machine.
// It'll receive the initial metadata of the request, call the underlying service's CopyFilesFromMachine
// and allow it to copy files into the RPC based FileCopier connected to the calling client.
func (server *serviceServer) CopyFilesFromMachine(srv pb.ShellService_CopyFilesFromMachineServer) error {
	mdReq, err := srv.Recv()
	if err != nil {
		return err
	}
	md, ok := mdReq.Request.(*pb.CopyFilesFromMachineRequest_Metadata)
	if !ok {
		return errors.New("expected copy request metadata")
	}
	svc, err := server.coll.Resource(md.Metadata.Name)
	if err != nil {
		return err
	}

	return svc.CopyFilesFromMachine(
		srv.Context(),
		md.Metadata.Paths,
		md.Metadata.AllowRecursion,
		md.Metadata.Preserve,
		newCopyFileFromMachineFactory(srv, md.Metadata.Preserve),
		md.Metadata.Extra.AsMap())
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
