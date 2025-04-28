package rpc

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/viamrobotics/webrtc/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/utils"
	webrtcpb "go.viam.com/utils/proto/rpc/webrtc/v1"
)

const testDelayAnswererNegotiationVar = "TEST_DELAY_ANSWERER_NEGOTIATION"

const heartbeatReceivedLog = "Received a heartbeat from the signaling server"

// A webrtcSignalingAnswerer listens for and answers calls with a given signaling service. It is
// directly connected to a Server that will handle the actual calls/connections over WebRTC
// data channels.
type webrtcSignalingAnswerer struct {
	startStopMu sync.Mutex // startStopMu guards the Start and Stop methods so they do not happen concurrently.

	address      string
	hosts        []string
	server       *webrtcServer
	dialOpts     []DialOption
	webrtcConfig webrtc.Configuration

	bgWorkers *utils.StoppableWorkers

	// conn is used to share the direct gRPC connection used by the answerer workers. As direct gRPC connections
	// reconnect on their own, custom reconnect logic is not needed. However, keepalives are necessary for the connection
	// to realize it's been disconnected quickly and start reconnecting. conn can be set to a pre-existing gRPC connection that's used by
	// other consumers via a dial option. In this scenario, sharedConn will be true, and the answerer will not attempt to establish a new
	// connection to the signaling server. If this option is not set, the answerer will oversee the lifecycle of its own connection by
	// continuously dialing in the background until a successful connection emerges and closing said connection when done. In the shared
	// connection case, the answerer will not close the connection.	connMu     sync.Mutex
	connMu     sync.Mutex
	conn       ClientConn
	sharedConn bool

	logger utils.ZapCompatibleLogger
}

// newWebRTCSignalingAnswerer makes an answerer that will connect to and listen for calls at the given
// address. Note that using this assumes that the connection at the given address is secure and
// assumed that all calls are authenticated. Random ports will be opened on this host to establish
// connections as a means to service ICE (https://webrtcforthecurious.com/docs/03-connecting/#how-does-it-work).
func newWebRTCSignalingAnswerer(
	address string,
	hosts []string,
	server *webrtcServer,
	dialOpts []DialOption,
	webrtcConfig webrtc.Configuration,
	logger utils.ZapCompatibleLogger,
) *webrtcSignalingAnswerer {
	dialOptsCopy := make([]DialOption, len(dialOpts))
	copy(dialOptsCopy, dialOpts)
	dialOptsCopy = append(dialOptsCopy, WithWebRTCOptions(DialWebRTCOptions{Disable: true}))
	options := &dialOptions{}
	for _, opt := range dialOptsCopy {
		opt.apply(options)
	}
	bgWorkers := utils.NewBackgroundStoppableWorkers()
	ans := &webrtcSignalingAnswerer{
		address:      address,
		hosts:        hosts,
		server:       server,
		dialOpts:     dialOptsCopy,
		webrtcConfig: webrtcConfig,
		bgWorkers:    bgWorkers,
		logger:       logger,
	}
	if options.signalingConn != nil {
		ans.conn = options.signalingConn
		ans.sharedConn = true
	}
	return ans
}

const (
	defaultMaxAnswerers               = 2
	answererConnectTimeout            = 10 * time.Second
	answererConnectTimeoutBehindProxy = time.Minute
	answererReconnectWait             = time.Second
)

// Start connects to the signaling service and listens forever until instructed to stop
// via Stop. Start cannot be called more than once before a Stop().
func (ans *webrtcSignalingAnswerer) Start() {
	ans.startStopMu.Lock()
	defer ans.startStopMu.Unlock()

	// attempt to make connection in a loop
	ans.bgWorkers.Add(func(ctx context.Context) {
		for ans.conn == nil {
			if ctx.Err() != nil {
				return
			}

			timeout := answererConnectTimeout
			// Bump timeout from 10 seconds to 1 minute if behind a SOCKS proxy. It
			// may take longer to connect to the signaling server in that case.
			if proxyAddr := os.Getenv(SocksProxyEnvVar); proxyAddr != "" {
				timeout = answererConnectTimeoutBehindProxy
			}
			setupCtx, timeoutCancel := context.WithTimeout(ctx, timeout)
			conn, err := Dial(setupCtx, ans.address, ans.logger, ans.dialOpts...)
			timeoutCancel()
			if err != nil {
				ans.logger.Errorw("error connecting answer client", "error", err)
				utils.SelectContextOrWait(ctx, answererReconnectWait)
				continue
			}
			ans.connMu.Lock()
			ans.conn = conn
			ans.connMu.Unlock()
		}
		// spin off the actual answerer workers
		for i := 0; i < defaultMaxAnswerers; i++ {
			ans.startAnswerer()
		}
	})
}

func isNetworkError(err error) bool {
	s, isGRPCErr := status.FromError(err)
	if err == nil || errors.Is(err, io.EOF) ||
		utils.FilterOutError(err, context.Canceled) == nil ||
		(isGRPCErr &&
			(s.Code() == codes.DeadlineExceeded ||
				s.Code() == codes.Canceled ||
				strings.Contains(s.Message(), "too_many_pings") ||
				// RSDK-3025: Cloud Run has a max one hour timeout which will terminate gRPC
				// streams, but leave the underlying connection open. That situation can
				// manifest in a few different errors (also see RSDK-10156.)
				strings.Contains(s.Message(), "upstream max stream duration reached") ||
				strings.Contains(s.Message(), "stream terminated by RST_STREAM") ||
				strings.Contains(s.Message(), "server closed the stream without sending trailers"))) {
		return false
	}
	return true
}

func (ans *webrtcSignalingAnswerer) startAnswerer() {
	newAnswer := func() (webrtcpb.SignalingService_AnswerClient, error) {
		ans.connMu.Lock()
		conn := ans.conn
		ans.connMu.Unlock()
		client := webrtcpb.NewSignalingServiceClient(conn)
		md := metadata.New(nil)
		md.Append(RPCHostMetadataField, ans.hosts...)
		md.Append(HeartbeatsAllowedMetadataField, "true")
		// use StoppableWorkers.Context() so that instantiation of answer client responds to StoppableWorkers.Stop()
		answerCtx := metadata.NewOutgoingContext(ans.bgWorkers.Context(), md)
		answerClient, err := client.Answer(answerCtx)
		if err != nil {
			return nil, err
		}
		return answerClient, nil
	}

	ans.bgWorkers.Add(func(ctx context.Context) {
		var client webrtcpb.SignalingService_AnswerClient
		defer func() {
			if client == nil {
				return
			}
			if err := client.CloseSend(); err != nil {
				ans.logger.Errorw("error closing send side of answering client", "error", err)
			}
		}()
		for {
			if ctx.Err() != nil {
				return
			}

			var err error
			// `newAnswer` opens a bidi grpc stream to the signaling server. But otherwise sends no requests.
			client, err = newAnswer()
			if err != nil {
				if isNetworkError(err) {
					ans.logger.Warnw("error communicating with signaling server", "error", err)
					utils.SelectContextOrWait(ctx, answererReconnectWait)
				}
				continue
			}

			var incomingCallerReq *webrtcpb.AnswerRequest
			for {
				// `client.Recv` waits, typically for a long time, for a caller to show
				// up. Which is when the signaling server will send a response saying
				// someone wants to connect. It can also receive heartbeats every 15s.
				//
				// The answerer does not respond to heartbeats. The signaling server is
				// only using heartbeats to ensure the answerer is reachable. If the
				// answerer is down, the heartbeat will error in the server's
				// heartbeating goroutine, the server's stream's context will be
				// canceled, and the server will stop handling interactions for this
				// answerer.
				incomingCallerReq, err = client.Recv()
				if err != nil {
					break
				}
				if _, ok := incomingCallerReq.GetStage().(*webrtcpb.AnswerRequest_Heartbeat); ok {
					ans.logger.Debug(heartbeatReceivedLog)
					continue
				}
				break // not a heartbeat
			}
			if err != nil {
				if isNetworkError(err) {
					ans.logger.Warnw("error communicating with signaling server", "error", err)
					utils.SelectContextOrWait(ctx, answererReconnectWait)
				}
				continue
			}

			// Create an `answerAttempt` to take advantage of the `sendError` method for the
			// upcoming type check.
			aa := &answerAttempt{
				webrtcSignalingAnswerer: ans,
				uuid:                    incomingCallerReq.GetUuid(),
				client:                  client,
				trickleEnabled:          true,
			}

			initStage, ok := incomingCallerReq.GetStage().(*webrtcpb.AnswerRequest_Init)
			if !ok {
				err := fmt.Errorf("expected first stage to be init or heartbeat; got %T", incomingCallerReq.GetStage())
				aa.sendError(err)
				ans.logger.Warn(err.Error())
				continue
			}

			if cfg := initStage.Init.GetOptionalConfig(); cfg != nil && cfg.GetDisableTrickle() {
				aa.trickleEnabled = false
			}
			aa.offerSDP = initStage.Init.GetSdp()

			var answerCtx context.Context
			var answerCtxCancel func()
			if deadline := initStage.Init.GetDeadline(); deadline != nil {
				answerCtx, answerCtxCancel = context.WithDeadline(ctx, deadline.AsTime())
			} else {
				answerCtx, answerCtxCancel = context.WithTimeout(ctx, getDefaultOfferDeadline())
			}

			if err = aa.connect(answerCtx); err != nil {
				answerCtxCancel()
				// We received an error while trying to connect to a caller/peer.
				ans.logger.Errorw("error connecting to peer", "error", err)
				utils.SelectContextOrWait(ctx, answererReconnectWait)
			}
			answerCtxCancel()
		}
	})
}

// Stop waits for the answer to stop listening and return.
func (ans *webrtcSignalingAnswerer) Stop() {
	ans.startStopMu.Lock()
	defer ans.startStopMu.Unlock()

	ans.bgWorkers.Stop()

	ans.connMu.Lock()
	defer ans.connMu.Unlock()
	if ans.conn != nil {
		if !ans.sharedConn {
			err := ans.conn.Close()
			if isNetworkError(err) {
				ans.logger.Errorw("error closing signaling connection", "error", err)
			}
		}
		ans.conn = nil
	}
}

type answerAttempt struct {
	*webrtcSignalingAnswerer
	// The uuid is the key for communicating with the signaling server about this connection
	// attempt.
	uuid   string
	client webrtcpb.SignalingService_AnswerClient

	trickleEnabled bool
	offerSDP       string

	// When a connection attempt concludes, either with success or failure, we will fire a single
	// message to the signaling server. This allows the signaling server to release resources
	// related to this connection attempt.
	sendDoneErrOnce sync.Once
}

// connect accepts a single call offer, responds with a corresponding SDP, and
// attempts to establish a WebRTC connection with the caller via ICE. Once established,
// the designated WebRTC data channel is passed off to the underlying Server which
// is then used as the server end of a gRPC connection.
func (aa *answerAttempt) connect(ctx context.Context) (err error) {
	connectionStartTime := time.Now()

	// Always extend WebRTC config with an `OptionalWebRTCConfig` call to the signaling server.
	// This allows the server to create a local TURN ICE candidate to
	// make a connection to any peer. Nomination of that type of candidate is only
	// possible through extending the WebRTC config with a TURN URL (and
	// associated username and password).
	webrtcConfig := aa.webrtcConfig
	aa.connMu.Lock()
	conn := aa.conn
	aa.connMu.Unlock()

	// Use first host on answerer for rpc-host field in metadata.
	signalingClient := webrtcpb.NewSignalingServiceClient(conn)
	md := metadata.New(map[string]string{RPCHostMetadataField: aa.hosts[0]})

	signalCtx := metadata.NewOutgoingContext(ctx, md)
	configResp, err := signalingClient.OptionalWebRTCConfig(signalCtx,
		&webrtcpb.OptionalWebRTCConfigRequest{})
	if err != nil {
		// Any error below indicates the signaling server is not present.
		if s, ok := status.FromError(err); ok && (s.Code() == codes.Unimplemented ||
			(s.Code() == codes.InvalidArgument && s.Message() == hostNotAllowedMsg)) {
			aa.server.counters.PeerConnectionErrors.Add(1)
			return ErrNoWebRTCSignaler
		}
		aa.server.counters.PeerConnectionErrors.Add(1)
		return err
	}
	webrtcConfig = extendWebRTCConfig(&webrtcConfig, configResp.GetConfig())
	iceUrls := make([]string, 0)
	for _, ice := range webrtcConfig.ICEServers {
		iceUrls = append(iceUrls, ice.URLs...)
	}
	aa.logger.Infow("extended WebRTC config", "ice servers", iceUrls)

	pc, dc, err := newPeerConnectionForServer(
		ctx,
		aa.offerSDP,
		webrtcConfig,
		!aa.trickleEnabled,
		aa.logger,
	)
	if err != nil {
		aa.sendError(err)
		aa.server.counters.PeerConnectionErrors.Add(1)
		return err
	}

	// We have a PeerConnection object. Install an error handler.
	var successful bool
	defer func() {
		if !(successful && err == nil) {
			var candPairStr string
			if candPair, hasCandPair := webrtcPeerConnCandPair(pc); hasCandPair {
				candPairStr = candPair.String()
			}

			connInfo := getWebRTCPeerConnectionStats(pc)
			iceConnectionState := pc.ICEConnectionState()
			iceGatheringState := pc.ICEGatheringState()
			aa.logger.Warnw("Connection establishment failed",
				"conn_id", connInfo.ID,
				"ice_connection_state", iceConnectionState,
				"ice_gathering_state", iceGatheringState,
				"conn_local_candidates", connInfo.LocalCandidates,
				"conn_remote_candidates", connInfo.RemoteCandidates,
				"candidate_pair", candPairStr,
			)

			// Close unhealthy connection.
			utils.UncheckedError(pc.GracefulClose())
		}
	}()

	serverChannel := aa.server.NewChannel(pc, dc, aa.hosts)

	initSent := make(chan struct{})
	if aa.trickleEnabled {
		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			aa.server.counters.PeerConnectionErrors.Add(1)
			return err
		}

		var pendingCandidates sync.WaitGroup
		waitOneHost := make(chan struct{})
		var waitOneHostOnce sync.Once
		pc.OnICECandidate(func(icecandidate *webrtc.ICECandidate) {
			if ctx.Err() != nil {
				return
			}
			if icecandidate != nil {
				pendingCandidates.Add(1)
				if icecandidate.Typ == webrtc.ICECandidateTypeHost {
					waitOneHostOnce.Do(func() {
						close(waitOneHost)
					})
				}
			}

			// must spin off to unblock the ICE gatherer
			aa.bgWorkers.Add(func(ctx context.Context) {
				if icecandidate != nil {
					defer pendingCandidates.Done()
				}

				select {
				case <-initSent:
				case <-ctx.Done():
					return
				}
				// there are no more candidates coming during this negotiation
				if icecandidate == nil {
					if _, ok := os.LookupEnv(testDelayAnswererNegotiationVar); ok {
						// RSDK-4293: Introducing a sleep here replicates the conditions
						// for a prior goroutine leak.
						aa.logger.Debug("Sleeping to delay the end of the negotiation")
						time.Sleep(1 * time.Second)
					}
					pendingCandidates.Wait()
					aa.sendDone()
					return
				}
				iProto := iceCandidateToProto(icecandidate)
				if err := aa.client.Send(&webrtcpb.AnswerResponse{
					Uuid: aa.uuid,
					Stage: &webrtcpb.AnswerResponse_Update{
						Update: &webrtcpb.AnswerResponseUpdateStage{
							Candidate: iProto,
						},
					},
				}); err != nil {
					aa.sendError(err)
				}
			})
		})

		err = pc.SetLocalDescription(answer)
		if err != nil {
			aa.server.counters.PeerConnectionErrors.Add(1)
			return err
		}

		select {
		case <-waitOneHost:
			// Dan: We wait for one host before proceeding to ensure the initial response has some
			// candidate information. This is a Nagle's algorithm-esque batching optimization. I
			// think.
		case <-ctx.Done():
			aa.server.counters.PeerConnectionErrors.Add(1)
			return ctx.Err()
		}
	}

	encodedSDP, err := EncodeSDP(pc.LocalDescription())
	if err != nil {
		aa.server.counters.PeerConnectionErrors.Add(1)
		aa.sendError(err)
		return err
	}

	if err := aa.client.Send(&webrtcpb.AnswerResponse{
		Uuid: aa.uuid,
		Stage: &webrtcpb.AnswerResponse_Init{
			Init: &webrtcpb.AnswerResponseInitStage{
				Sdp: encodedSDP,
			},
		},
	}); err != nil {
		aa.server.counters.PeerConnectionErrors.Add(1)
		return err
	}
	close(initSent)

	if aa.trickleEnabled {
		done := make(chan struct{})
		defer func() { <-done }()

		utils.PanicCapturingGoWithCallback(func() {
			defer close(done)

			for {
				// `client` was constructed based off of the `ans.closeCtx`. We rely on the
				// underlying `client.Recv` implementation checking that context for cancelation.
				ansResp, err := aa.client.Recv()
				if err != nil {
					if !errors.Is(err, io.EOF) {
						aa.logger.Warn("Error receiving initial message from signaling server", "err", err)
					}
					return
				}

				switch stage := ansResp.GetStage().(type) {
				case *webrtcpb.AnswerRequest_Init:
				case *webrtcpb.AnswerRequest_Update:
					if ansResp.GetUuid() != aa.uuid {
						aa.sendError(fmt.Errorf("uuid mismatch; have=%q want=%q", ansResp.GetUuid(), aa.uuid))
						return
					}
					cand := iceCandidateFromProto(stage.Update.GetCandidate())
					if err := pc.AddICECandidate(cand); err != nil {
						aa.sendError(err)
						return
					}
				case *webrtcpb.AnswerRequest_Done:
					return
				case *webrtcpb.AnswerRequest_Error:
					respStatus := status.FromProto(stage.Error.GetStatus())
					aa.sendError(fmt.Errorf("error from requester: %w", respStatus.Err()))
					return
				case *webrtcpb.AnswerRequest_Heartbeat:
					aa.logger.Debug(heartbeatReceivedLog)
				default:
					aa.sendError(fmt.Errorf("unexpected stage %T", stage))
					return
				}
			}
		}, func(err interface{}) {
			aa.sendError(fmt.Errorf("%v", err))
		})
	}

	select {
	case <-serverChannel.Ready():
		// Happy path
		successful = true
		aa.server.counters.PeersConnected.Add(1)
		aa.server.counters.TotalTimeConnectingMillis.Add(time.Since(connectionStartTime).Milliseconds())
	case <-ctx.Done():
		// Timed out or signaling server was closed.
		serverChannel.Close()
		aa.sendError(ctx.Err())
		aa.server.counters.PeerConnectionErrors.Add(1)
		return ctx.Err()
	}

	aa.sendDone()
	return nil
}

func (aa *answerAttempt) sendDone() {
	aa.sendDoneErrOnce.Do(func() {
		sendErr := aa.client.Send(&webrtcpb.AnswerResponse{
			Uuid: aa.uuid,
			Stage: &webrtcpb.AnswerResponse_Done{
				Done: &webrtcpb.AnswerResponseDoneStage{},
			},
		})

		if sendErr != nil {
			// Errors communicating with the signaling server have no bearing on whether the
			// PeerConnection is usable. Log and ignore the send error.
			aa.logger.Warnw("Failed to send connection success message to signaling server", "sendErr", sendErr)
		}
	})
}

func (aa *answerAttempt) sendError(err error) {
	aa.sendDoneErrOnce.Do(func() {
		sendErr := aa.client.Send(&webrtcpb.AnswerResponse{
			Uuid: aa.uuid,
			Stage: &webrtcpb.AnswerResponse_Error{
				Error: &webrtcpb.AnswerResponseErrorStage{
					Status: ErrorToStatus(err).Proto(),
				},
			},
		})

		if sendErr != nil {
			aa.logger.Warnw("Failed to send error message to signaling server", "sendErr", sendErr)
		}
	})
}
