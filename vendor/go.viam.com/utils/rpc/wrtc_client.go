package rpc

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/viamrobotics/webrtc/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/utils"
	webrtcpb "go.viam.com/utils/proto/rpc/webrtc/v1"
)

// ErrNoWebRTCSignaler happens if a gRPC request is made on a server that does not support
// signaling for WebRTC or explicitly not the host requested.
var ErrNoWebRTCSignaler = errors.New("no signaler present")

// DialWebRTCOptions control how WebRTC is utilized in a dial attempt.
type DialWebRTCOptions struct {
	// Disable prevents a WebRTC connection attempt.
	Disable bool

	// SignalingInsecure determines if the signaling connection is insecure.
	SignalingInsecure bool

	// SignalingServerAddress specifies the signaling server to
	// contact on behalf of this client for WebRTC communications.
	SignalingServerAddress string

	// SignalingAuthEntity is the entity to authenticate as to the signaler.
	SignalingAuthEntity string

	// SignalingExternalAuthAddress is the address to perform external auth yet.
	// This is unlikely to be needed since the signaler is typically in the same
	// place where authentication happens.
	SignalingExternalAuthAddress string

	// SignalingExternalAuthToEntity is the entity to authenticate for after
	// externally authenticating.
	// This is unlikely to be needed since the signaler is typically in the same
	// place where authentication happens.
	SignalingExternalAuthToEntity string

	// SignalingExternalAuthInsecure is whether or not the external auth server
	// is insecure.
	// This is unlikely to be needed since the signaler is typically in the same
	// place where authentication happens.
	SignalingExternalAuthInsecure bool

	// SignalingCreds are used to authenticate the request to the signaling server.
	SignalingCreds Credentials

	// SignalingExternalAuthAuthMaterial is used when the credentials for the signaler
	// have already been used to exchange an auth payload. In those cases this can be set
	// to bypass the Authenticate/AuthenticateTo rpc auth flow.
	SignalingExternalAuthAuthMaterial string

	// DisableTrickleICE controls whether to disable Trickle ICE or not.
	// Disabling Trickle ICE can slow down connection establishment.
	DisableTrickleICE bool

	// Config is the WebRTC specific configuration (i.e. ICE settings)
	Config *webrtc.Configuration

	// AllowAutoDetectAuthOptions allows authentication options to be automatically
	// detected. Only use this if you trust the signaling server.
	AllowAutoDetectAuthOptions bool
}

// DialWebRTC connects to the signaling service at the given address and attempts to establish
// a WebRTC connection with the corresponding peer reflected in the address.
// It provider client/server functionality for gRPC serviced over
// WebRTC data channels. The work is adapted from https://github.com/jsmouret/grpc-over-webrtc.
func DialWebRTC(
	ctx context.Context,
	signalingServer string,
	host string,
	logger utils.ZapCompatibleLogger,
	opts ...DialOption,
) (conn ClientConn, err error) {
	var dOpts dialOptions
	for _, opt := range opts {
		opt.apply(&dOpts)
	}
	dOpts.webrtcOpts.Disable = false
	dOpts.webrtcOpts.SignalingServerAddress = signalingServer
	return dialInner(ctx, host, logger, dOpts)
}

func dialWebRTC(
	ctx context.Context,
	signalingServer string,
	host string,
	dOpts dialOptions,
	logger utils.ZapCompatibleLogger,
) (*webrtcClientChannel, error) {
	dialStart := time.Now()

	dialCtx, timeoutCancel := context.WithTimeout(ctx, getDefaultOfferDeadline())
	defer timeoutCancel()

	logger.Debugw(
		"connecting to signaling server",
		"signaling_server", signalingServer,
		"host", host,
	)

	conn, err := dialSignalingServer(dialCtx, signalingServer, host, logger, dOpts)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Ignore any errors closing the signaling server connection. That step has no bearing on
		// whether the PeerConnection was successfully made.
		utils.UncheckedError(conn.Close())
	}()

	logger.Debugw("connected to signaling server", "signaling_server", signalingServer)

	md := metadata.New(map[string]string{RPCHostMetadataField: host})
	signalCtx := metadata.NewOutgoingContext(dialCtx, md)

	signalingClient := webrtcpb.NewSignalingServiceClient(conn)
	configResp, err := signalingClient.OptionalWebRTCConfig(signalCtx, &webrtcpb.OptionalWebRTCConfigRequest{})
	if err != nil {
		// this would be where we would hit an unimplemented signaler error first.
		if s, ok := status.FromError(err); ok && (s.Code() == codes.Unimplemented ||
			(s.Code() == codes.InvalidArgument && s.Message() == hostNotAllowedMsg)) {
			return nil, ErrNoWebRTCSignaler
		}
		return nil, err
	}

	config := DefaultWebRTCConfiguration
	if dOpts.webrtcOpts.Config != nil {
		config = *dOpts.webrtcOpts.Config
	}
	extendedConfig := extendWebRTCConfig(&config, configResp.GetConfig())
	peerConn, dataChannel, err := newPeerConnectionForClient(ctx, extendedConfig, dOpts.webrtcOpts.DisableTrickleICE, logger)
	if err != nil {
		return nil, err
	}

	var (
		statsMu                                        sync.Mutex
		callUpdates                                    int
		maxCallUpdateDuration, totalCallUpdateDuration time.Duration
	)
	onICEConnected := func() {
		// Delay by up to 5s to allow more caller updates/better stats.
		waitTime := 5 * time.Second
		if testing.Testing() {
			waitTime = 100 * time.Millisecond
		}
		select {
		case <-time.After(waitTime):
		case <-ctx.Done():
		}

		statsMu.Lock()
		defer statsMu.Unlock()
		if callUpdates == 0 {
			return
		}
		averageCallUpdateDuration := totalCallUpdateDuration / time.Duration(callUpdates)
		// TODO: Potentially report these stats to sentry/some central location at some point.
		logger.Debugw("ICE connected", "time_since_dial_start_ms", time.Since(dialStart).Milliseconds(), "num_call_updates",
			callUpdates, "average_duration_ms", averageCallUpdateDuration.Milliseconds(), "max_call_update_duration_ms",
			maxCallUpdateDuration.Milliseconds())
	}

	//nolint:contextcheck
	clientCh := newWebRTCClientChannel(peerConn,
		dataChannel,
		onICEConnected,
		utils.Sublogger(logger, "client"),
		dOpts.unaryInterceptor,
		dOpts.streamInterceptor)

	var successful bool
	defer func() {
		if !successful {
			clientCh.close()
			utils.UncheckedError(peerConn.GracefulClose())
		}
	}()

	exchangeCtx, exchangeCancel := context.WithCancelCause(signalCtx)

	// bool representing whether initial sdp exchange has occurred
	haveInit := false

	var uuid string
	// only send once since exchange may end or ICE may end
	var sendDoneOnce sync.Once
	sendDone := func() {
		sendDoneOnce.Do(func() {
			if _, err = signalingClient.CallUpdate(signalCtx, &webrtcpb.CallUpdateRequest{
				Uuid: uuid,
				Update: &webrtcpb.CallUpdateRequest_Done{
					Done: true,
				},
			}); err != nil {
				logger.Warnw("Error sending CallUpdate", "err", err)
			}
		})
	}

	// this channel blocks goroutines spawned for each ICE candidate in OnIceCandidate from sending a CallUpdateRequest
	// to the signaling server until a CallResponse_Init is received, which in turn causes the channel to be closed and
	// unblocks goroutines from sending candidate update requests
	remoteDescSet := make(chan struct{})

	if !dOpts.webrtcOpts.DisableTrickleICE {
		offer, err := peerConn.CreateOffer(nil)
		if err != nil {
			return nil, err
		}

		var pendingCandidates sync.WaitGroup

		// waitOneHost is closed when the first ICE candidate of type `Host` (e.g: 127.0.0.1) is
		// found.
		waitOneHost := make(chan struct{})
		var waitOneHostOnce sync.Once
		peerConn.OnICECandidate(func(icecandidate *webrtc.ICECandidate) {
			if exchangeCtx.Err() != nil {
				// Caller has canceled the dial, or a timeout has occurred.
				return
			}

			if icecandidate != nil {
				// The last `icecandidate` called from pion will be nil. `nil` signifies that all
				// candidates were created. We will still create a goroutine for this "empty"
				// candidate to wait for all other candidates to complete. Thus we only increment
				// `pendingCandidates` for non-nil values.
				pendingCandidates.Add(1)
				if icecandidate.Typ == webrtc.ICECandidateTypeHost {
					waitOneHostOnce.Do(func() {
						close(waitOneHost)
					})
				}
			}

			// must spin off to unblock the ICE gatherer
			utils.PanicCapturingGo(func() {
				if icecandidate != nil {
					defer pendingCandidates.Done()
				}
				select {
				case <-remoteDescSet:
					// We've received the `init` answer and initialized `uuid`. We can now proceed
					// with sending individual candidates.
				case <-exchangeCtx.Done():
					return
				}

				if icecandidate == nil {
					// There are no more candidates to generate. Wait for all existing
					// candidates/CallUpdate's to complete. Then "sendDone".
					pendingCandidates.Wait()
					sendDone()
					return
				}

				iProto := iceCandidateToProto(icecandidate)
				callUpdateStart := time.Now()
				if _, err := signalingClient.CallUpdate(exchangeCtx, &webrtcpb.CallUpdateRequest{
					Uuid: uuid,
					Update: &webrtcpb.CallUpdateRequest_Candidate{
						Candidate: iProto,
					},
				}); err != nil {
					logger.Warnw("Error sending a CallUpdate", "err", err)
					return
				}

				statsMu.Lock()
				callUpdates++
				callUpdateDuration := time.Since(callUpdateStart)
				if callUpdateDuration > maxCallUpdateDuration {
					maxCallUpdateDuration = callUpdateDuration
				}
				totalCallUpdateDuration += time.Since(callUpdateStart)
				statsMu.Unlock()
			})
		})

		err = peerConn.SetLocalDescription(offer)
		if err != nil {
			logger.Errorw("Error setting local description with offer", "err", err)
			return nil, err
		}

		select {
		case <-exchangeCtx.Done():
			logger.Errorw("Failed while waiting for first host to be generated", "err", err)
			return nil, exchangeCtx.Err()
		case <-waitOneHost:
		}
	}

	encodedSDP, err := EncodeSDP(peerConn.LocalDescription())
	if err != nil {
		logger.Errorw("Error encoding local description", "err", err)
		return nil, err
	}

	callClient, err := signalingClient.Call(signalCtx, &webrtcpb.CallRequest{Sdp: encodedSDP})
	if err != nil {
		logger.Errorw("Error calling with initial SDP", "err", err)
		return nil, err
	}

	// TODO(RSDK-245): do separate auth here
	if dOpts.externalAuthAddr != "" { //nolint:revive
		// TODO(RSDK-245): prepare AuthenticateTo here
		// for client channel.
	} else if dOpts.creds.Type != "" { //nolint:staticcheck,revive
		// TODO(RSDK-245): prepare Authenticate here
		// for client channel
	}

	exchangeCandidates := func() error {
		for {
			if err := exchangeCtx.Err(); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}

			callResp, err := callClient.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}

				return err
			}
			switch s := callResp.GetStage().(type) {
			case *webrtcpb.CallResponse_Init:
				if haveInit {
					return errors.New("got init stage more than once")
				}
				haveInit = true
				uuid = callResp.GetUuid()
				answer := webrtc.SessionDescription{}
				if err := DecodeSDP(s.Init.GetSdp(), &answer); err != nil {
					return err
				}

				err = peerConn.SetRemoteDescription(answer)
				if err != nil {
					return err
				}
				close(remoteDescSet)

				if dOpts.webrtcOpts.DisableTrickleICE {
					sendDone()
					return nil
				}
			case *webrtcpb.CallResponse_Update:
				if !haveInit {
					return errors.New("got update stage before init stage")
				}
				if callResp.GetUuid() != uuid {
					return errors.Errorf("uuid mismatch; have=%q want=%q", callResp.GetUuid(), uuid)
				}
				cand := iceCandidateFromProto(s.Update.GetCandidate())
				if err := peerConn.AddICECandidate(cand); err != nil {
					// A PeerConnection only needs one valid candidate to succeed. It's unclear why
					// only some* candidates would be malformed, so we'll log, but otherwise ignore.
					logger.Warnw("Error adding candidate", "err", err)
					continue
				}
			default:
				return errors.Errorf("unexpected stage %T", s)
			}
		}
	}

	utils.PanicCapturingGo(func() {
		if err := exchangeCandidates(); err != nil {
			logger.Warnw("Failed to exchange candidates", "err", err)
			exchangeCancel(err)
		}
	})

	select {
	case <-clientCh.Ready():
		// Happy path
		sendDone()
		successful = true

		// Ensure the exchange goroutine has exited.
		exchangeCancel(nil)
		<-exchangeCtx.Done()
	case <-exchangeCtx.Done():
		exchangeErr := context.Cause(exchangeCtx)
		sendDoneOnce.Do(func() {
			if _, err = signalingClient.CallUpdate(signalCtx, &webrtcpb.CallUpdateRequest{
				Uuid: uuid,
				Update: &webrtcpb.CallUpdateRequest_Error{
					Error: ErrorToStatus(exchangeErr).Proto(),
				},
			}); err != nil {
				logger.Warnw("Problem sending error to signaling server", "err", err)
			}
		})
		return nil, exchangeErr
	}

	return clientCh, nil
}

func dialSignalingServer(
	ctx context.Context,
	signalingServer string,
	host string,
	logger utils.ZapCompatibleLogger,
	dOpts dialOptions,
) (ClientConn, error) {
	dOpts.insecure = dOpts.webrtcOpts.SignalingInsecure

	// replace auth entity and creds
	dOpts.authEntity = dOpts.webrtcOpts.SignalingAuthEntity
	dOpts.creds = dOpts.webrtcOpts.SignalingCreds
	dOpts.externalAuthAddr = dOpts.webrtcOpts.SignalingExternalAuthAddress
	dOpts.externalAuthToEntity = dOpts.webrtcOpts.SignalingExternalAuthToEntity
	dOpts.externalAuthInsecure = dOpts.webrtcOpts.SignalingExternalAuthInsecure
	dOpts.externalAuthMaterial = dOpts.webrtcOpts.SignalingExternalAuthAuthMaterial

	// ignore AuthEntity when auth material is available.
	if dOpts.authEntity == "" {
		if dOpts.externalAuthAddr == "" {
			// if we are not doing external auth, then the entity is assumed to be the actual host.
			if dOpts.debug {
				logger.Debugw("auth entity empty; setting to host", "host", host)
			}
			dOpts.authEntity = host
		} else {
			// otherwise it's the external auth address.
			if dOpts.debug {
				logger.Debugw("auth entity empty; setting to external auth address", "address", dOpts.externalAuthAddr)
			}
			dOpts.authEntity = dOpts.externalAuthAddr
		}
	}

	conn, _, err := dialDirectGRPC(ctx, signalingServer, dOpts, logger)
	return conn, err
}
