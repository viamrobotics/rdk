package session

import (
	"context"
	"crypto/subtle"
	"sync"
	"time"

	"github.com/google/uuid"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/resource"
)

// A Session allows a client to express that it is actively connected and
// supports stopping actuating components when it's not.
type Session struct {
	mu              sync.Mutex
	id              uuid.UUID
	peerConnInfo    *pb.PeerConnectionInfo
	ownerID         []byte
	deadline        time.Time
	heartbeatWindow time.Duration

	associateResource func(id uuid.UUID, resourceName resource.Name)
}

// New makes a new session.
func New(
	ctx context.Context,
	ownerID string,
	heartbeatWindow time.Duration,
	associateResource func(id uuid.UUID, resourceName resource.Name),
) *Session {
	return NewWithID(ctx, uuid.New(), ownerID, heartbeatWindow, associateResource)
}

// NewWithID makes a new session with an ID.
func NewWithID(
	ctx context.Context,
	id uuid.UUID,
	ownerID string,
	heartbeatWindow time.Duration,
	associateResource func(id uuid.UUID, resourceName resource.Name),
) *Session {
	sess := &Session{
		id:                id,
		ownerID:           []byte(ownerID),
		heartbeatWindow:   heartbeatWindow,
		associateResource: associateResource,
	}
	sess.Heartbeat(ctx)
	return sess
}

// ID returns the id of this session.
func (s *Session) ID() uuid.UUID {
	return s.id
}

// CheckOwnerID checks if the given owner is the same as the one on this session.
func (s *Session) CheckOwnerID(against string) bool {
	return subtle.ConstantTimeCompare([]byte(against), s.ownerID) == 1
}

// Heartbeat signals a single heartbeat to the session.
func (s *Session) Heartbeat(ctx context.Context) {
	s.mu.Lock()
	s.deadline = time.Now().Add(s.heartbeatWindow)
	s.peerConnInfo = peerConnectionInfoToProto(rpc.PeerConnectionInfoFromContext(ctx))
	s.mu.Unlock()
}

// Active checks if this session is still active.
func (s *Session) Active(at time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deadline.After(at)
}

// PeerConnectionInfo returns connection info related to the session.
func (s *Session) PeerConnectionInfo() *pb.PeerConnectionInfo {
	return s.peerConnInfo
}

// HeartbeatWindow returns the time window that a single heartbeat must sent within.
func (s *Session) HeartbeatWindow() time.Duration {
	return s.heartbeatWindow
}

// Deadline returns when this session is set to expire.
func (s *Session) Deadline() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deadline
}

func (s *Session) associateWith(targetName resource.Name) {
	if !s.Active(time.Now()) {
		return
	}
	if s.associateResource != nil {
		s.associateResource(s.ID(), targetName)
	}
}

func peerConnectionInfoToProto(info rpc.PeerConnectionInfo) *pb.PeerConnectionInfo {
	var connType pb.PeerConnectionType
	switch info.ConnectionType {
	case rpc.PeerConnectionTypeGRPC:
		connType = pb.PeerConnectionType_PEER_CONNECTION_TYPE_GRPC
	case rpc.PeerConnectionTypeWebRTC:
		connType = pb.PeerConnectionType_PEER_CONNECTION_TYPE_WEBRTC
	case rpc.PeerConnectionTypeUnknown:
		fallthrough
	default:
		connType = pb.PeerConnectionType_PEER_CONNECTION_TYPE_UNSPECIFIED
	}

	pbInfo := &pb.PeerConnectionInfo{
		Type: connType,
	}
	if info.LocalAddress != "" {
		pbInfo.LocalAddress = &info.LocalAddress
	}
	if info.RemoteAddress != "" {
		pbInfo.RemoteAddress = &info.RemoteAddress
	}
	return pbInfo
}
