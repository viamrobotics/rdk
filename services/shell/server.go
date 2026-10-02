package shell

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"strings"
	"syscall"
	"time"

	"github.com/viamrobotics/webrtc/v3"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/shell/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the contract from shell.proto.
type serviceServer struct {
	pb.UnimplementedShellServiceServer
	coll   resource.APIResourceGetter[Service]
	logger logging.Logger
}

// NewRPCServiceServer constructs a framesystem gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Service], logger logging.Logger) interface{} {
	// Scope copy/shell diagnostics under a "shell" sublogger so they're easy to find and filter on
	// the machine Logs page, rather than emitting under the bare root logger.
	if logger != nil {
		logger = logger.Sublogger("shell")
	}
	return &serviceServer{coll: coll, logger: logger}
}

// copyTransportSnapshot reads the WebRTC/SCTP transport state for the connection serving ctx, if
// it is a WebRTC connection. connID matches the key used for this connection in the server's FTDC,
// so a copy log can be correlated to that connection's cwnd/rwnd series. Returns ok=false for a
// non-WebRTC (direct/local) connection, which has no such stats.
func copyTransportSnapshot(ctx context.Context) (connID string, bytesSent, bytesReceived uint64, ok bool) {
	peerConn, has := rpc.ContextPeerConnection(ctx)
	if !has || peerConn == nil {
		return "", 0, 0, false
	}
	for _, stat := range peerConn.GetStats() {
		switch typed := stat.(type) {
		case webrtc.PeerConnectionStats:
			connID = typed.ID
		case webrtc.SCTPTransportStats:
			bytesSent, bytesReceived, ok = typed.BytesSent, typed.BytesReceived, true
		}
	}
	return connID, bytesSent, bytesReceived, ok && connID != ""
}

// logCopySummary emits a single structured line per file-copy RPC so copies are visible in
// machine logs and can be correlated with the per-connection SCTP stats in FTDC
// via conn_id. transportBytes is the SCTP byte delta over the copy (received for a copy to the
// machine, sent for a copy from it); transport fields are omitted for non-WebRTC connections.
func (server *serviceServer) logCopySummary(
	direction, target string, preserve bool, dur time.Duration, connID string, transportBytes uint64, haveTransport bool, copyErr error,
) {
	if server.logger == nil {
		return
	}
	keysAndValues := []any{
		"direction", direction,
		"target", target,
		"preserve", preserve,
		"duration_ms", dur.Milliseconds(),
	}
	if haveTransport {
		var mbps float64
		if secs := dur.Seconds(); secs > 0 {
			mbps = math.Round(float64(transportBytes)/secs/1e6*1000) / 1000
		}
		keysAndValues = append(keysAndValues,
			"conn_id", connID, "transport_bytes", transportBytes, "throughput_mbps", mbps)
	}
	if copyErr != nil {
		server.logger.Warnw("shell file copy failed", append(keysAndValues, "error", copyErr)...)
		return
	}
	server.logger.Infow("shell file copy completed", keysAndValues...)
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
		md.Metadata.Extra.AsMap(),
	)
	if err != nil {
		var pathErr *fs.PathError
		var errno syscall.Errno
		if errors.As(err, &pathErr) && errors.As(pathErr.Err, &errno) && errno == syscall.EACCES {
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

	start := time.Now()
	connID, _, receivedBefore, okBefore := copyTransportSnapshot(srv.Context())
	copyErr := reader.ReadAll(srv.Context())
	_, _, receivedAfter, okAfter := copyTransportSnapshot(srv.Context())
	server.logCopySummary("to_machine", md.Metadata.Destination, md.Metadata.Preserve, time.Since(start),
		connID, receivedAfter-receivedBefore, okBefore && okAfter, copyErr)
	return copyErr
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

	start := time.Now()
	connID, sentBefore, _, okBefore := copyTransportSnapshot(srv.Context())
	copyErr := svc.CopyFilesFromMachine(
		srv.Context(),
		md.Metadata.Paths,
		md.Metadata.AllowRecursion,
		md.Metadata.Preserve,
		newCopyFileFromMachineFactory(srv, md.Metadata.Preserve),
		md.Metadata.Extra.AsMap(),
	)
	_, sentAfter, _, okAfter := copyTransportSnapshot(srv.Context())
	server.logCopySummary("from_machine", strings.Join(md.Metadata.Paths, ","), md.Metadata.Preserve, time.Since(start),
		connID, sentAfter-sentBefore, okBefore && okAfter, copyErr)
	return copyErr
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

// GetStatus returns the status of the shell service.
func (server *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.GetStatusFromResourceServer(ctx, res, req)
}
