package robot

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/session"
)

func (m *SessionManager) safetyMonitoredTypeAndMethod(method string) (*resource.RPCAPI, *desc.MethodDescriptor, bool) {
	subType, methodDesc, err := TypeAndMethodDescFromMethod(m.robot, method)
	if err != nil {
		return nil, nil, false
	}
	var safetyHeartbeatMonitored bool
	if methodDesc != nil {
		unwrapped := methodDesc.UnwrapMethod()
		safetyHeartbeatMonitored = isSafetyHeartbeatMonitored(&unwrapped)
	}
	return subType, methodDesc, safetyHeartbeatMonitored
}

// isSafetyHeartbeatMonitored looks for an RPC method's safety_heartbeat_monitored option and returns its value, or false if unspecified.
func isSafetyHeartbeatMonitored(descriptor *protoreflect.MethodDescriptor) bool {
	if descriptor == nil {
		return false
	}
	opts := (*descriptor).Options()
	if proto.HasExtension(opts, commonpb.E_SafetyHeartbeatMonitored) {
		return proto.GetExtension(opts, commonpb.E_SafetyHeartbeatMonitored).(bool)
	}
	return false
}

// IsSafetyHeartbeatMonitored looks for an RPC method's safety_heartbeat_monitored option and returns its value, or false if unspecified.
func IsSafetyHeartbeatMonitored(method string) bool {
	// reformat "/viam.component.base.v1.BaseService/MoveStraight" -> "viam.component.base.v1.BaseService.MoveStraight"
	method = strings.TrimPrefix(method, "/")
	method = strings.ReplaceAll(method, "/", ".")
	// err is NotFound if not present. We just need to return false in this case.
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(method))
	if err == nil {
		if md, ok := descriptor.(protoreflect.MethodDescriptor); ok {
			return isSafetyHeartbeatMonitored(&md)
		}
	}
	return false
}

func (m *SessionManager) safetyMonitoredResourceFromUnary(req interface{}, method string) resource.Name {
	subType, _, ok := m.safetyMonitoredTypeAndMethod(method)
	if !ok {
		return resource.Name{}
	}

	reqMsg := protoutils.MessageToProtoV1(req)
	if reqMsg == nil {
		return resource.Name{}
	}

	msg, err := dynamic.AsDynamicMessage(reqMsg)
	if err != nil {
		m.logger.Errorw("error converting message to dynamic", "error", err, "method", method)
		return resource.Name{}
	}

	_, resName, err := ResourceFromProtoMessage(m.robot, msg, subType.API)
	if err != nil {
		m.logger.Errorw("unable to find resource", "error", err)
		return resource.Name{}
	}
	return resName
}

type firstMessageServerStreamWrapper struct {
	mu sync.Mutex
	grpc.ServerStream
	firstMsg *dynamic.Message
}

func (w *firstMessageServerStreamWrapper) RecvMsg(m interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.firstMsg != nil {
		msgTarget := protoutils.MessageToProtoV1(m)
		if msgTarget == nil {
			return errors.Errorf("expected message to be a proto.Message but got %T", m)
		}
		msg := w.firstMsg
		w.firstMsg = nil
		return msg.ConvertToDeterministic(msgTarget)
	}
	return w.ServerStream.RecvMsg(m)
}

func (m *SessionManager) safetyMonitoredResourceFromStream(
	stream grpc.ServerStream,
	method string,
) (resource.Name, grpc.ServerStream, error) {
	subType, methodDesc, ok := m.safetyMonitoredTypeAndMethod(method)
	if !ok {
		// Note(erd): could maybe cache this in the future but may be subject to a DOS attack
		// since method space is unbounded.
		return resource.Name{}, nil, nil
	}

	firstMsg := dynamic.NewMessage(methodDesc.GetInputType())

	if err := stream.RecvMsg(firstMsg); err != nil {
		// this error counts
		return resource.Name{}, nil, err
	}

	newStream := &firstMessageServerStreamWrapper{ServerStream: stream, firstMsg: firstMsg}

	_, resName, err := ResourceFromProtoMessage(m.robot, firstMsg, subType.API)
	if err != nil {
		m.logger.Errorw("unable to find resource", "error", err)
		return resource.Name{}, newStream, nil
	}

	return resName, newStream, nil
}

// ServerInterceptors returns gRPC interceptors to work with sessions.
func (m *SessionManager) ServerInterceptors() session.ServerInterceptors {
	return session.ServerInterceptors{
		UnaryServerInterceptor:  m.UnaryServerInterceptor,
		StreamServerInterceptor: m.StreamServerInterceptor,
	}
}

// UnaryServerInterceptor associates the current session (if present) in the current context before
// passing it to the unary response handler.
func (m *SessionManager) UnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	if _, _, isMonitored := m.safetyMonitoredTypeAndMethod(info.FullMethod); !isMonitored {
		return handler(ctx, req)
	}
	safetyMonitoredResourceName := m.safetyMonitoredResourceFromUnary(req, info.FullMethod)
	ctx, err := associateSession(ctx, m, safetyMonitoredResourceName, info.FullMethod)
	if err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// StreamServerInterceptor associates the current session (if present) in the current context before
// passing it to the stream response handler.
func (m *SessionManager) StreamServerInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	if _, _, isMonitored := m.safetyMonitoredTypeAndMethod(info.FullMethod); !isMonitored {
		return handler(srv, ss)
	}
	safetyMonitoredResource, wrappedStream, err := m.safetyMonitoredResourceFromStream(ss, info.FullMethod)
	if err != nil {
		return err
	}
	if wrappedStream != nil {
		ss = wrappedStream
	}
	ctx, err := associateSession(ss.Context(), m, safetyMonitoredResource, info.FullMethod)
	if err != nil {
		return err
	}
	return handler(srv, &ssStreamContextWrapper{ss, ctx})
}

// associateSession creates a new context associated with the session, if found, from an incoming context.
func associateSession(
	ctx context.Context,
	m *SessionManager,
	safetyMonitoredResourceName resource.Name,
	method string,
) (nextCtx context.Context, err error) {
	var sessID uuid.UUID
	if safetyMonitoredResourceName != (resource.Name{}) {
		// defer this because no matter what we want to know that someone was using
		// a resource with a monitored method as long as no error happened.
		defer func() {
			if err == nil {
				m.AssociateResource(sessID, safetyMonitoredResourceName)
			}
		}()
	}
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		m.logger.CWarnw(ctx, "failed to pull metadata from context", "method", method)
		return ctx, nil
	}
	sessID, err = sessionFromMetadata(meta)
	if err != nil {
		m.logger.CWarnw(ctx, "failed to get session id from metadata", "error", err)
		return ctx, err
	}
	if sessID == uuid.Nil {
		return ctx, nil
	}
	authEntity, _ := rpc.ContextAuthEntity(ctx)
	sess, err := m.FindByID(ctx, sessID, authEntity.Entity)
	if err != nil {
		return nil, err
	}
	return session.ToContext(ctx, sess), nil
}

// sessionFromMetadata returns a session id from metadata.
func sessionFromMetadata(meta metadata.MD) (uuid.UUID, error) {
	values := meta.Get(session.IDMetadataKey)
	switch len(values) {
	case 0:
		return uuid.UUID{}, nil
	case 1:
		sessID, err := uuid.Parse(values[0])
		if err != nil {
			return uuid.UUID{}, err
		}
		return sessID, nil
	default:
		return uuid.UUID{}, errors.New("found more than one session id in metadata")
	}
}

type ssStreamContextWrapper struct {
	grpc.ServerStream
	ctx context.Context
}

func (w ssStreamContextWrapper) Context() context.Context {
	return w.ctx
}
