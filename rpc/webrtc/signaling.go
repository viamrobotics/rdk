package rpcwebrtc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/edaniels/golog"
	gwebrtc "github.com/edaniels/gostream/webrtc"
	"go.uber.org/multierr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	webrtcpb "go.viam.com/core/proto/rpc/webrtc/v1"
	"go.viam.com/core/rpc/dialer"
	"go.viam.com/core/utils"
)

// A SignalingServer implements a signaling service for WebRTC by exchanging
// SDPs (https://webrtcforthecurious.com/docs/02-signaling/#what-is-the-session-description-protocol-sdp)
// via gRPC. The service consists of a many-to-many interaction where there are many callers
// and many answerers. The callers provide an SDP to the service which asks a corresponding
// waiting answerer to provide an SDP in exchange in order to establish a P2P connection between
// the two parties.
type SignalingServer struct {
	webrtcpb.UnimplementedSignalingServiceServer
	callQueue *MemoryCallQueue
}

// NewSignalingServer makes a new signaling server that uses an in memory
// call queue and looks routes based on a given robot host.
// TODO(https://github.com/viamrobotics/core/issues/79): abstraction to be able to use
// MongoDB as a distributed call queue. This will enable many signaling services to
// run acting as effectively operators on as switchboard.
func NewSignalingServer() *SignalingServer {
	return &SignalingServer{callQueue: NewMemoryCallQueue()}
}

// RPCHostMetadataField is the identifier of a host.
const RPCHostMetadataField = "rpc-host"

func hostFromCtx(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md[RPCHostMetadataField]) == 0 {
		return "", fmt.Errorf("expected %s to be set in metadata", RPCHostMetadataField)
	}
	host := md[RPCHostMetadataField][0]
	if host == "" {
		return "", fmt.Errorf("expected non-empty %s", RPCHostMetadataField)
	}
	return host, nil
}

// Call is a request/offer to start a caller with the connected answerer.
func (srv *SignalingServer) Call(ctx context.Context, req *webrtcpb.CallRequest) (*webrtcpb.CallResponse, error) {
	host, err := hostFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	respSDP, err := srv.callQueue.SendOffer(ctx, host, req.Sdp)
	if err != nil {
		return nil, err
	}
	return &webrtcpb.CallResponse{Sdp: respSDP}, nil
}

// Answer listens on call/offer queue forever responding with SDPs to agreed to calls.
// TODO(https://github.com/viamrobotics/core/issues/104): This should be authorized for robots only.
func (srv *SignalingServer) Answer(server webrtcpb.SignalingService_AnswerServer) error {
	ctx := server.Context()
	host, err := hostFromCtx(ctx)
	if err != nil {
		return err
	}

	for {
		offer, err := srv.callQueue.RecvOffer(ctx, host)
		if err != nil {
			return err
		}
		ans, cont := func() (CallAnswer, bool) {
			if err := server.Send(&webrtcpb.AnswerRequest{Sdp: offer.SDP()}); err != nil {
				return CallAnswer{Err: err}, false
			}
			answer, err := server.Recv()
			if err != nil {
				return CallAnswer{Err: err}, false
			}
			respStatus := status.FromProto(answer.Status)
			if respStatus.Code() != codes.OK {
				return CallAnswer{Err: respStatus.Err()}, true
			}
			return CallAnswer{SDP: answer.Sdp}, true
		}()
		if err := offer.Respond(ctx, ans); err != nil {
			return err
		}
		if !cont {
			return ans.Err
		}
	}
}

// A SignalingAnswerer listens for and answers calls with a given signaling service. It is
// directly connected to a Server that will handle the actual calls/connections over WebRTC
// data channels.
type SignalingAnswerer struct {
	address                 string
	host                    string
	client                  webrtcpb.SignalingService_AnswerClient
	server                  *Server
	insecure                bool
	activeBackgroundWorkers sync.WaitGroup
	cancelBackgroundWorkers func()
	closeCtx                context.Context
	logger                  golog.Logger
}

// NewSignalingAnswerer makes an answerer that will connect to and listen for calls at the given
// address. Note that using this assumes that the connection at the given address is secure and
// assumed that all calls are authenticated. Random ports will be opened on this host to establish
// connections as a means to service ICE (https://webrtcforthecurious.com/docs/03-connecting/#how-does-it-work).
func NewSignalingAnswerer(address, host string, server *Server, insecure bool, logger golog.Logger) *SignalingAnswerer {
	closeCtx, cancel := context.WithCancel(context.Background())
	return &SignalingAnswerer{
		address:                 address,
		host:                    host,
		server:                  server,
		insecure:                insecure,
		cancelBackgroundWorkers: cancel,
		closeCtx:                closeCtx,
		logger:                  logger,
	}
}

const answererReconnectWait = time.Second

// Start connects to the signaling service and listens forever until instructed to stop
// via Stop.
func (ans *SignalingAnswerer) Start() error {
	var connInUse dialer.ClientConn
	var connMu sync.Mutex
	connect := func() error {
		connMu.Lock()
		conn := connInUse
		connMu.Unlock()
		if conn != nil {
			if err := conn.Close(); err != nil {
				ans.logger.Errorw("error closing existing signaling connection", "error", err)
			}
		}
		setupCtx, timeoutCancel := context.WithTimeout(ans.closeCtx, 5*time.Second)
		defer timeoutCancel()
		conn, err := dialer.DialDirectGRPC(setupCtx, ans.address, ans.insecure)
		if err != nil {
			return err
		}
		connMu.Lock()
		connInUse = conn
		connMu.Unlock()

		client := webrtcpb.NewSignalingServiceClient(conn)
		md := metadata.New(map[string]string{RPCHostMetadataField: ans.host})
		answerCtx := metadata.NewOutgoingContext(ans.closeCtx, md)
		answerClient, err := client.Answer(answerCtx)
		if err != nil {
			return multierr.Combine(err, conn.Close())
		}
		ans.client = answerClient
		return nil
	}
	if err := connect(); err != nil {
		return err
	}

	ans.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			select {
			case <-ans.closeCtx.Done():
				return
			default:
			}
			if err := ans.answer(); err != nil && utils.FilterOutError(err, context.Canceled) != nil {
				ans.logger.Errorw("error answering", "error", err)
				for {
					ans.logger.Debugw("reconnecting answer client", "in", answererReconnectWait.String())
					if !utils.SelectContextOrWait(ans.closeCtx, answererReconnectWait) {
						return
					}
					if err := connect(); err != nil {
						ans.logger.Errorw("error reconnecting answer client", "error", err)
						continue
					}
					ans.logger.Debug("reconnected answer client")
					break
				}
			}
		}
	}, func() {
		defer ans.activeBackgroundWorkers.Done()
		defer func() {
			connMu.Lock()
			conn := connInUse
			connMu.Unlock()
			if err := conn.Close(); err != nil {
				ans.logger.Errorw("error closing signaling connection", "error", err)
			}
		}()
		defer func() {
			if err := ans.client.CloseSend(); err != nil {
				ans.logger.Errorw("error closing send side of answering client", "error", err)
			}
		}()
	})

	return nil
}

// Stop waits for the answer to stop listening and return.
func (ans *SignalingAnswerer) Stop() {
	ans.cancelBackgroundWorkers()
	ans.activeBackgroundWorkers.Wait()
}

// answer accepts a single call offer, responds with a corresponding SDP, and
// attempts to establish a WebRTC connection with the caller via ICE. Once established,
// the designated WebRTC data channel is passed off to the underlying Server which
// is then used as the server end of a gRPC connection.
func (ans *SignalingAnswerer) answer() (err error) {
	resp, err := ans.client.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	pc, dc, err := newPeerConnectionForServer(ans.closeCtx, resp.Sdp, ans.logger)
	if err != nil {
		return ans.client.Send(&webrtcpb.AnswerResponse{
			Status: ErrorToStatus(err).Proto(),
		})
	}
	var successful bool
	defer func() {
		if !(successful && err == nil) {
			err = multierr.Combine(err, pc.Close())
		}
	}()

	encodedSDP, err := gwebrtc.EncodeSDP(pc.LocalDescription())
	if err != nil {
		return ans.client.Send(&webrtcpb.AnswerResponse{
			Status: ErrorToStatus(err).Proto(),
		})
	}

	ans.server.NewChannel(pc, dc)

	successful = true
	return ans.client.Send(&webrtcpb.AnswerResponse{
		Status: ErrorToStatus(nil).Proto(),
		Sdp:    encodedSDP,
	})
}
