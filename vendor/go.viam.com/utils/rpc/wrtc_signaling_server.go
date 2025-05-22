package rpc

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/utils"
	webrtcpb "go.viam.com/utils/proto/rpc/webrtc/v1"
)

// A WebRTCSignalingServer implements a signaling service for WebRTC by exchanging
// SDPs (https://webrtcforthecurious.com/docs/02-signaling/#what-is-the-session-description-protocol-sdp)
// via gRPC. The service consists of a many-to-many interaction where there are many callers
// and many answerers. The callers provide an SDP to the service which asks a corresponding
// waiting answerer to provide an SDP in exchange in order to establish a P2P connection between
// the two parties.
// Note: authorization should happen by something wrapping this service server.
type WebRTCSignalingServer struct {
	webrtcpb.UnimplementedSignalingServiceServer
	mu                   sync.RWMutex
	callQueue            WebRTCCallQueue
	hostICEServers       map[string]hostICEServers
	webrtcConfigProvider WebRTCConfigProvider
	forHosts             map[string]struct{}

	bgWorkers *utils.StoppableWorkers

	logger utils.ZapCompatibleLogger

	// Interval at which to send heartbeats.
	heartbeatInterval time.Duration
}

// NewWebRTCSignalingServer makes a new signaling server that uses the given
// call queue and looks routes based on a given robot host. If forHosts is
// non-empty, the server will only accept the given hosts and reject all
// others. The signaling server will send heartbeats to answerers at the
// provided heartbeatInterval if the answerer requests heartbeats through
// the initial Answer metadata.
func NewWebRTCSignalingServer(
	callQueue WebRTCCallQueue,
	webrtcConfigProvider WebRTCConfigProvider,
	logger utils.ZapCompatibleLogger,
	heartbeatInterval time.Duration,
	forHosts ...string,
) *WebRTCSignalingServer {
	forHostsSet := make(map[string]struct{}, len(forHosts))
	for _, host := range forHosts {
		forHostsSet[host] = struct{}{}
	}

	bgWorkers := utils.NewBackgroundStoppableWorkers()
	return &WebRTCSignalingServer{
		callQueue:            callQueue,
		hostICEServers:       map[string]hostICEServers{},
		webrtcConfigProvider: webrtcConfigProvider,
		forHosts:             forHostsSet,
		bgWorkers:            bgWorkers,
		logger:               logger,
		heartbeatInterval:    heartbeatInterval,
	}
}

// RPCHostMetadataField is the identifier of a host.
const RPCHostMetadataField = "rpc-host"

// HeartbeatsAllowedMetadataField is the identifier for allowing heartbeats
// from a signaling server to answerers.
const HeartbeatsAllowedMetadataField = "heartbeats-allowed"

// Default interval at which to send heartbeats.
const defaultHeartbeatInterval = 15 * time.Second

// HostFromCtx gets the host being called/answered for from the context.
func HostFromCtx(ctx context.Context) (string, error) {
	hosts, err := HostsFromCtx(ctx)
	if err != nil {
		return "", err
	}
	if len(hosts) != 1 {
		return "", fmt.Errorf("expected 1 %s", RPCHostMetadataField)
	}
	return hosts[0], nil
}

const maxHostsInMetadata = 5

// HostsFromCtx gets the hosts being called/answered for from the context.
func HostsFromCtx(ctx context.Context) ([]string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md[RPCHostMetadataField]) == 0 {
		return nil, fmt.Errorf("expected %s to be set in metadata", RPCHostMetadataField)
	}
	if len(md[RPCHostMetadataField]) > maxHostsInMetadata {
		return nil, fmt.Errorf("too many %s", RPCHostMetadataField)
	}

	hostsCopy := make([]string, len(md[RPCHostMetadataField]))
	copy(hostsCopy, md[RPCHostMetadataField])
	for _, host := range hostsCopy {
		if host == "" {
			return nil, fmt.Errorf("expected non-empty %s", RPCHostMetadataField)
		}
	}
	return hostsCopy, nil
}

const hostNotAllowedMsg = "host not preconfigured"

func (srv *WebRTCSignalingServer) validateHosts(hosts ...string) error {
	if len(srv.forHosts) == 0 {
		return nil
	}
	if len(hosts) == 0 {
		return errors.New("at least one host required")
	}
	for _, host := range hosts {
		if _, ok := srv.forHosts[host]; ok {
			continue
		}
		return status.Error(codes.InvalidArgument, hostNotAllowedMsg)
	}
	return nil
}

// HeartbeatsAllowedFromCtx checks if heartbeats are allowed with respect to
// the context.
func HeartbeatsAllowedFromCtx(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md[HeartbeatsAllowedMetadataField]) == 0 {
		return false
	}
	// Only allow "true" as a value for now.
	return md[HeartbeatsAllowedMetadataField][0] == "true"
}

func (srv *WebRTCSignalingServer) asyncSendOfferError(host, uuid string, offerErr error) {
	srv.bgWorkers.Add(func(ctx context.Context) {
		// Use a timeout to not block shutdown.
		sendCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		sendErr := srv.callQueue.SendOfferError(sendCtx, host, uuid, offerErr)
		if sendErr == nil {
			return
		}

		var errInactive inactiveOfferError
		if !errors.As(sendErr, &errInactive) {
			srv.logger.Warnw("error sending offer error", "host", host, "id", uuid, "offerErr", offerErr, "sendErr", sendErr)
		}
	})
}

// Call is a request/offer to start a caller with the connected answerer.
func (srv *WebRTCSignalingServer) Call(req *webrtcpb.CallRequest, server webrtcpb.SignalingService_CallServer) error {
	ctx := server.Context()
	ctx, cancel := context.WithTimeout(ctx, getDefaultOfferDeadline())
	defer cancel()

	host, err := HostFromCtx(ctx)
	if err != nil {
		return err
	}
	if err := srv.validateHosts(host); err != nil {
		return err
	}
	uuid, respCh, respDone, sendCancel, err := srv.callQueue.SendOfferInit(ctx, host, req.GetSdp(), req.GetDisableTrickle())
	if err != nil {
		return err
	}
	defer sendCancel()

	var haveInit bool
	for {
		var resp WebRTCCallAnswer
		select {
		case <-ctx.Done():
			srv.asyncSendOfferError(host, uuid, context.Cause(ctx))
			return ctx.Err()
		case <-respDone:
			return nil
		case resp = <-respCh:
		}
		if resp.Err != nil {
			err := fmt.Errorf("error from answerer: %w", resp.Err)
			srv.asyncSendOfferError(host, uuid, err)
			return err
		}

		if !haveInit && resp.InitialSDP == nil {
			err := errors.New("expected to have initial SDP if no error")
			srv.asyncSendOfferError(host, uuid, err)
			return err
		}
		if !haveInit {
			haveInit = true
			if err := server.Send(&webrtcpb.CallResponse{
				Uuid: uuid,
				Stage: &webrtcpb.CallResponse_Init{
					Init: &webrtcpb.CallResponseInitStage{
						Sdp: *resp.InitialSDP,
					},
				},
			}); err != nil {
				srv.asyncSendOfferError(host, uuid, err)
				return err
			}
		}

		if resp.Candidate == nil {
			continue
		}

		ip := iceCandidateInitToProto(*resp.Candidate)
		if err := server.Send(&webrtcpb.CallResponse{
			Uuid: uuid,
			Stage: &webrtcpb.CallResponse_Update{
				Update: &webrtcpb.CallResponseUpdateStage{
					Candidate: ip,
				},
			},
		}); err != nil {
			// Don't set an error for the connection attempt. Some candidates may have been
			// exchanged that can result in a successful connection.
			return err
		}
	}
}

// CallUpdate is used to send additional info in relation to a Call.
// In a world where https://github.com/grpc/grpc-web/issues/24 is fixed,
// this should be removed in favor of a bidirectional stream on Call.
func (srv *WebRTCSignalingServer) CallUpdate(ctx context.Context, req *webrtcpb.CallUpdateRequest) (*webrtcpb.CallUpdateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, getDefaultOfferDeadline())
	defer cancel()
	host, err := HostFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := srv.validateHosts(host); err != nil {
		return nil, err
	}
	switch u := req.GetUpdate().(type) {
	case *webrtcpb.CallUpdateRequest_Candidate:
		cand := iceCandidateFromProto(u.Candidate)
		if err := srv.callQueue.SendOfferUpdate(ctx, host, req.GetUuid(), cand); err != nil {
			return nil, err
		}
	case *webrtcpb.CallUpdateRequest_Error:
		if err := srv.callQueue.SendOfferError(ctx, host, req.GetUuid(), status.ErrorProto(req.GetError())); err != nil {
			return nil, err
		}
	case *webrtcpb.CallUpdateRequest_Done:
		if err := srv.callQueue.SendOfferDone(ctx, host, req.GetUuid()); err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("unknown update stage %T", u)
	}
	return &webrtcpb.CallUpdateResponse{}, nil
}

type hostICEServers struct {
	Servers []*webrtcpb.ICEServer
	Expires time.Time
}

func (srv *WebRTCSignalingServer) additionalICEServers(ctx context.Context, hosts []string, cache bool) ([]*webrtcpb.ICEServer, error) {
	if srv.webrtcConfigProvider == nil {
		return nil, nil
	}
	hostsKey := strings.Join(hosts, ",")
	srv.mu.RLock()
	hostServers := srv.hostICEServers[hostsKey]
	srv.mu.RUnlock()
	if time.Now().Before(hostServers.Expires) {
		return hostServers.Servers, nil
	}
	config, err := srv.webrtcConfigProvider.Config(ctx)
	if err != nil {
		return nil, err
	}
	if cache {
		srv.mu.Lock()
		srv.hostICEServers[hostsKey] = hostICEServers{
			Servers: config.ICEServers,
			Expires: config.Expires,
		}
		srv.mu.Unlock()
	}
	return config.ICEServers, nil
}

// Note: We expect but do not enforce one host for one answer. If this is not true, a race
// can happen where we may double fetch additional ICE servers.
func (srv *WebRTCSignalingServer) clearAdditionalICEServers(hosts []string) {
	srv.mu.Lock()
	for _, host := range hosts {
		delete(srv.hostICEServers, host)
	}
	srv.mu.Unlock()
}

// Answer listens on call/offer queue for a single call responding with a corresponding SDP
// and candidate updates/errors.
// Note: See SinalingAnswer.answer for the complementary side of this process.
func (srv *WebRTCSignalingServer) Answer(server webrtcpb.SignalingService_AnswerServer) error {
	ctx := server.Context()
	hosts, err := HostsFromCtx(ctx)
	if err != nil {
		return err
	}
	if err := srv.validateHosts(hosts...); err != nil {
		return err
	}
	defer srv.clearAdditionalICEServers(hosts)

	// If heartbeats allowed (indicated by answerer), start goroutine to send
	// heartbeats.
	//
	// The answerer does not respond to heartbeats. The signaling server is only
	// using heartbeats to ensure the answerer is reachable. If the answerer is
	// down, the heartbeat will error in the heartbeating goroutine below, the
	// stream's context will be canceled, and we will stop handling interactions
	// for this answerer. We stop handling interactions because the stream's
	// context (`ctx` here and below) is used in the `RecvOffer` call below this
	// goroutine that waits for a caller to attempt to establish a connection.
	if HeartbeatsAllowedFromCtx(ctx) {
		utils.PanicCapturingGo(func() {
			for {
				select {
				case <-time.After(srv.heartbeatInterval):
					if err := server.Send(&webrtcpb.AnswerRequest{
						Stage: &webrtcpb.AnswerRequest_Heartbeat{},
					}); err != nil {
						srv.logger.Debugw(
							"error sending answer heartbeat",
							"error", err,
						)
					}
				case <-ctx.Done():
					return
				}
			}
		})
	}

	offer, err := srv.callQueue.RecvOffer(ctx, hosts)
	if err != nil {
		return err
	}

	iceServers, err := srv.additionalICEServers(ctx, hosts, true)
	if err != nil {
		return err
	}

	// initialize
	uuid := offer.UUID()
	if err := server.Send(&webrtcpb.AnswerRequest{
		Uuid: uuid,
		Stage: &webrtcpb.AnswerRequest_Init{
			Init: &webrtcpb.AnswerRequestInitStage{
				Sdp: offer.SDP(),
				OptionalConfig: &webrtcpb.WebRTCConfig{
					AdditionalIceServers: iceServers,
					DisableTrickle:       offer.DisableTrickleICE(),
				},
				Deadline: timestamppb.New(offer.Deadline()),
			},
		},
	}); err != nil {
		return err
	}

	offerCtx, offerCtxCancel := context.WithDeadline(ctx, offer.Deadline())
	var answererStoppedExchange atomic.Bool
	callerLoop := func() error {
		defer func() {
			if !answererStoppedExchange.Load() {
				if err := server.Send(&webrtcpb.AnswerRequest{
					Uuid: uuid,
					Stage: &webrtcpb.AnswerRequest_Done{
						Done: &webrtcpb.AnswerRequestDoneStage{},
					},
				}); err != nil {
					srv.logger.Debugw(
						"error sending answer request done",
						"uuid", uuid,
						"error", err,
					)
				}
			}
		}()
		for {
			select {
			case <-offerCtx.Done():
				return offerCtx.Err()
			case <-offer.CallerDone():
				callerErr := offer.CallerErr()
				if callerErr != nil {
					if err := server.Send(&webrtcpb.AnswerRequest{
						Uuid: uuid,
						Stage: &webrtcpb.AnswerRequest_Error{
							Error: &webrtcpb.AnswerRequestErrorStage{
								Status: ErrorToStatus(callerErr).Proto(),
							},
						},
					}); err != nil {
						return multierr.Combine(callerErr, err)
					}
				}
				return callerErr
			case cand := <-offer.CallerCandidates():
				ip := iceCandidateInitToProto(cand)
				if err := server.Send(&webrtcpb.AnswerRequest{
					Uuid: uuid,
					Stage: &webrtcpb.AnswerRequest_Update{
						Update: &webrtcpb.AnswerRequestUpdateStage{
							Candidate: ip,
						},
					},
				}); err != nil {
					return err
				}
			}
		}
	}

	answererLoop := func() error {
		haveInit := false
		for {
			answer, err := server.Recv()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return err
				}
				return nil
			}

			if answer.GetUuid() != uuid {
				return errors.Errorf("uuid mismatch; have=%q want=%q", answer.GetUuid(), uuid)
			}

			switch s := answer.GetStage().(type) {
			case *webrtcpb.AnswerResponse_Init:
				if haveInit {
					return errors.New("got init stage more than once")
				}
				haveInit = true
				init := s.Init

				ans := WebRTCCallAnswer{InitialSDP: &init.Sdp}
				if err := offer.AnswererRespond(server.Context(), ans); err != nil {
					return err
				}
			case *webrtcpb.AnswerResponse_Update:
				if !haveInit {
					return errors.New("got update stage before init stage")
				}
				cand := iceCandidateFromProto(s.Update.GetCandidate())
				if err := offer.AnswererRespond(server.Context(), WebRTCCallAnswer{
					Candidate: &cand,
				}); err != nil {
					return err
				}
			case *webrtcpb.AnswerResponse_Done:
				if !haveInit {
					return errors.New("got done stage before init stage")
				}
				return nil
			case *webrtcpb.AnswerResponse_Error:
				respStatus := status.FromProto(s.Error.GetStatus())
				ans := WebRTCCallAnswer{Err: respStatus.Err()}
				answererStoppedExchange.Store(true)
				offerCtxCancel() // and stop exchange
				return offer.AnswererRespond(server.Context(), ans)
			default:
				return errors.Errorf("unexpected stage %T", s)
			}
		}
	}

	callerErrCh := make(chan error, 1)
	srv.bgWorkers.Add(func(ctx context.Context) {
		defer func() {
			close(callerErrCh)
		}()
		if err := callerLoop(); err != nil {
			callerErrCh <- err
		}
	})

	// ensure we wait on the error channel
	return func() (err error) {
		defer func() {
			err = multierr.Combine(err, <-callerErrCh)
		}()
		defer func() {
			err = multierr.Combine(err, offer.AnswererDone(server.Context()))
		}()
		defer func() {
			if err != nil {
				// one side failed, cancel the other
				offerCtxCancel()
			}
		}()
		return answererLoop()
	}()
}

// OptionalWebRTCConfig returns any WebRTC configuration the caller may want to use.
func (srv *WebRTCSignalingServer) OptionalWebRTCConfig(
	ctx context.Context,
	req *webrtcpb.OptionalWebRTCConfigRequest,
) (*webrtcpb.OptionalWebRTCConfigResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, getDefaultOfferDeadline())
	defer cancel()
	hosts, err := HostsFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := srv.validateHosts(hosts...); err != nil {
		return nil, err
	}
	iceServers, err := srv.additionalICEServers(ctx, hosts, false)
	if err != nil {
		return nil, err
	}
	return &webrtcpb.OptionalWebRTCConfigResponse{Config: &webrtcpb.WebRTCConfig{
		AdditionalIceServers: iceServers,
	}}, nil
}

// Close cancels all active workers and waits to cleanly close all background workers.
func (srv *WebRTCSignalingServer) Close() {
	srv.bgWorkers.Stop()
}
