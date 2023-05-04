package session

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"go.viam.com/rdk/resource"
)

// A Manager holds sessions for a particular robot and manages their lifetime.
type Manager interface {
	Start(ctx context.Context, ownerID string) (*Session, error)
	All() []*Session
	FindByID(ctx context.Context, id uuid.UUID, ownerID string) (*Session, error)
	AssociateResource(id uuid.UUID, resourceName resource.Name)
	Close()

	// ServerInterceptors returns gRPC interceptors to work with sessions.
	ServerInterceptors() ServerInterceptors
}

// ServerInterceptors provide gRPC interceptors to work with sessions.
type ServerInterceptors struct {
	UnaryServerInterceptor func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error)
	StreamServerInterceptor func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error
}
