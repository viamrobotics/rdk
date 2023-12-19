package client

import (
	"context"
	"sync"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/session"
)

type ctxKey byte

const ctxKeyInSessionMDReq = ctxKey(iota)

var exemptFromSession = map[string]bool{
	"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo": true,
	"/proto.rpc.webrtc.v1.SignalingService/Call":                     true,
	"/proto.rpc.webrtc.v1.SignalingService/CallUpdate":               true,
	"/proto.rpc.webrtc.v1.SignalingService/OptionalWebRTCConfig":     true,
	"/proto.rpc.v1.AuthService/Authenticate":                         true,
	"/proto.rpc.v1.ExternalAuthService/AuthenticateTo":               true,
	"/viam.robot.v1.RobotService/ResourceNames":                      true,
	"/viam.robot.v1.RobotService/ResourceRPCSubtypes":                true,
	"/viam.robot.v1.RobotService/StartSession":                       true,
	"/viam.robot.v1.RobotService/SendSessionHeartbeat":               true,
}

func (rc *RobotClient) sessionReset() {
	rc.sessionMu.Lock()
	rc.sessionsSupported = nil
	rc.sessionMu.Unlock()
}

func (rc *RobotClient) heartbeatLoop() {
	rc.heartbeatWorkers.Add(1)
	utils.ManagedGo(func() {
		rc.sessionMu.RLock()
		ticker := time.NewTicker(rc.sessionHeartbeatInterval)
		rc.sessionMu.RUnlock()
		defer ticker.Stop()

		for {
			if !utils.SelectContextOrWaitChan(rc.heartbeatCtx, ticker.C) {
				return
			}

			rc.sessionMu.RLock()
			if rc.sessionsSupported == nil {
				// due to how this works, there may be more than one heartbeat loop going
				// but that's deemed fine for now.
				rc.sessionMu.RUnlock()
				return
			}
			sessID := rc.currentSessionID
			rc.sessionMu.RUnlock()

			sendReq := &pb.SendSessionHeartbeatRequest{
				Id: sessID,
			}
			if _, err := rc.client.SendSessionHeartbeat(rc.heartbeatCtx, sendReq); err != nil {
				if s, ok := status.FromError(err); ok && s.Code() == codes.Unavailable {
					rc.sessionReset()
					return
				}
				if !(utils.FilterOutError(err, context.Canceled) == nil ||
					utils.FilterOutError(err, context.DeadlineExceeded) == nil) {
					// this could be a session expiration but we will handle that via a retry
					// in the interceptors below
					rc.logger.Errorw("error sending heartbeat", "error", err)
					return
				}
				return
			}
		}
	}, rc.heartbeatWorkers.Done)
}

func (rc *RobotClient) sessionMetadataInner(ctx context.Context) context.Context {
	if *rc.sessionsSupported && rc.currentSessionID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, session.IDMetadataKey, rc.currentSessionID)
	}
	return ctx
}

func (rc *RobotClient) sessionMetadata(ctx context.Context, method string) (context.Context, error) {
	if !rc.useSessionInRequest(ctx, method) {
		return ctx, nil
	}
	ctx = context.WithValue(ctx, ctxKeyInSessionMDReq, true)
	rc.sessionMu.RLock()
	if rc.sessionsSupported != nil {
		defer rc.sessionMu.RUnlock()
		return rc.sessionMetadataInner(ctx), nil
	}
	rc.sessionMu.RUnlock()

	rc.sessionMu.Lock()
	defer rc.sessionMu.Unlock()

	// check one more time
	if rc.sessionsSupported != nil {
		return rc.sessionMetadataInner(ctx), nil
	}

	reqCtx, cancel := utils.MergeContext(ctx, rc.backgroundCtx)
	defer cancel()

	var startReq pb.StartSessionRequest
	if rc.currentSessionID != "" {
		startReq.Resume = rc.currentSessionID
	}

	startResp, err := rc.client.StartSession(
		reqCtx,
		&startReq,
		grpc_retry.WithMax(5),
		grpc_retry.WithCodes(codes.InvalidArgument),
	)
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Unimplemented {
			falseVal := false
			rc.sessionsSupported = &falseVal
			rc.logger.CInfow(ctx, "sessions unsupported; will not try again")
			return ctx, nil
		}
		return nil, err
	}

	heartbeatWindow := startResp.HeartbeatWindow.AsDuration()
	sessionHeartbeatInterval := heartbeatWindow / 5
	if heartbeatWindow <= 0 || sessionHeartbeatInterval <= 0 {
		rc.logger.CInfow(ctx, "session heartbeat window invalid; will not try again", "heartbeat_window", heartbeatWindow)
		return ctx, nil
	}

	trueVal := true
	rc.sessionsSupported = &trueVal
	rc.currentSessionID = startResp.Id
	rc.sessionHeartbeatInterval = sessionHeartbeatInterval
	rc.heartbeatLoop()

	return rc.sessionMetadataInner(ctx), nil
}

func (rc *RobotClient) safetyMonitorFromHeaders(ctx context.Context, hdr metadata.MD) {
	for _, name := range hdr.Get(session.SafetyMonitoredResourceMetadataKey) {
		resName, err := resource.NewFromString(name)
		if err != nil {
			rc.logger.Errorw("bad resource name from metadata", "error", err)
			continue
		}
		if rc.remoteName != "" {
			resName = resName.PrependRemote(rc.remoteName)
		}
		session.SafetyMonitorResourceName(ctx, resName)
	}
}

func (rc *RobotClient) useSessionInRequest(ctx context.Context, method string) bool {
	return !rc.sessionsDisabled && !exemptFromSession[method] && ctx.Value(ctxKeyInSessionMDReq) == nil
}

func (rc *RobotClient) sessionUnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	var hdr metadata.MD
	if rc.remoteName != "" {
		defer func() {
			rc.safetyMonitorFromHeaders(ctx, hdr)
		}()
	}
	invoke := func() error {
		ctx, err := rc.sessionMetadata(ctx, method)
		if err != nil {
			return err
		}
		return invoker(ctx, method, req, reply, cc, append(opts, grpc.Header(&hdr))...)
	}
	if !rc.useSessionInRequest(ctx, method) {
		// we won't retry but we will pass along any metadata we get from the remote to our parent(s).
		return invoke()
	}
	if err := invoke(); err != nil {
		if isStatusNoSessionError(err) {
			// Note(erd): This could cause a few sessions to start if the timing is bad
			// and others are calling sessionReset.Wwe may want to address this in the future.
			rc.sessionReset()
			// retry once more
			return invoke()
		}
		return err
	}
	return nil
}

func isStatusNoSessionError(err error) bool {
	if s, ok := status.FromError(err); ok &&
		s.Code() == session.StatusNoSession.Code() && s.Message() == session.StatusNoSession.Message() {
		return true
	}
	return false
}

type firstMessageClientStreamWrapper struct {
	grpc.ClientStream

	rc                       *RobotClient
	invoke                   func() (grpc.ClientStream, error)
	safetyMonitorFromHeaders func(hdr metadata.MD)

	mu        sync.RWMutex
	sendMsgs  []interface{}
	closeSend bool
	firstRecv bool
}

func (w *firstMessageClientStreamWrapper) SendMsg(m interface{}) error {
	w.mu.Lock()
	if !w.firstRecv {
		w.sendMsgs = append(w.sendMsgs, m)
	}
	w.mu.Unlock()
	return w.ClientStream.SendMsg(m)
}

func (w *firstMessageClientStreamWrapper) CloseSend() error {
	w.mu.Lock()
	w.closeSend = true
	w.mu.Unlock()
	return w.ClientStream.CloseSend()
}

func (w *firstMessageClientStreamWrapper) RecvMsg(m interface{}) error {
	w.mu.Lock()
	if w.firstRecv {
		w.mu.Unlock()
		return w.ClientStream.RecvMsg(m)
	}
	w.firstRecv = true
	w.mu.Unlock()

	if w.safetyMonitorFromHeaders != nil {
		defer func() {
			// do this last in case we hit a retry
			if md, err := w.ClientStream.Header(); err == nil {
				w.safetyMonitorFromHeaders(md)
			}
		}()
	}

	err := w.ClientStream.RecvMsg(m)
	if err == nil {
		w.mu.Lock()
		w.sendMsgs = nil // release
		w.mu.Unlock()
		return nil
	}

	if isStatusNoSessionError(err) {
		// Note(erd): This could cause a few sessions to start if the timing is bad
		// and others are calling sessionReset.Wwe may want to address this in the future.
		w.rc.sessionReset()

		// retry once more
		innnerStream, err := w.invoke()
		if err != nil {
			return err
		}

		w.ClientStream = innnerStream

		// replay
		w.mu.RLock()
		sendMsgs := w.sendMsgs
		closeSend := w.closeSend
		w.mu.RUnlock()
		for _, msg := range sendMsgs {
			if err := w.ClientStream.SendMsg(msg); err != nil {
				return err
			}
		}
		w.mu.Lock()
		w.sendMsgs = nil // release
		w.mu.Unlock()
		if closeSend {
			if err := w.ClientStream.CloseSend(); err != nil {
				return err
			}
		}

		return w.ClientStream.RecvMsg(m)
	}

	return err
}

func (rc *RobotClient) sessionStreamClientInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (cs grpc.ClientStream, err error) {
	invoke := func() (grpc.ClientStream, error) {
		ctx, sessErr := rc.sessionMetadata(ctx, method)
		if sessErr != nil {
			return nil, sessErr
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
	useSession := rc.useSessionInRequest(ctx, method)

	invokeCS, err := func() (grpc.ClientStream, error) {
		invokeCS, invokeErr := invoke()
		if !useSession {
			return invokeCS, invokeErr
		}
		if invokeErr != nil {
			if isStatusNoSessionError(err) {
				// Note(erd): this should never happen based on how gRPC streams work thus far
				// but it does not hurt to check in case I am wrong :)
				rc.sessionReset()
				// retry once more
				return invoke()
			}
			return nil, invokeErr
		}
		return invokeCS, nil
	}()
	if err != nil {
		return nil, err
	}
	wrapper := &firstMessageClientStreamWrapper{
		ClientStream: invokeCS,
		rc:           rc,
		invoke:       invoke,
	}
	if rc.remoteName != "" {
		wrapper.safetyMonitorFromHeaders = func(hdr metadata.MD) {
			rc.safetyMonitorFromHeaders(ctx, hdr)
		}
	}
	return wrapper, nil
}
