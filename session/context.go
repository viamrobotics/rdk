package session

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/resource"
)

var (
	// StatusNoSession is returned when a session has expired or does not exist.
	StatusNoSession = status.New(codes.InvalidArgument, "SESSION_EXPIRED")

	// ErrNoSession is returned when a session has expired or does not exist.
	ErrNoSession = StatusNoSession.Err()
)

const (
	// IDMetadataKey is the gRPC metadata key to use when transmitting session information.
	IDMetadataKey = "viam-sid"

	// SafetyMonitoredResourceMetadataKey is the gRPC metadata key to use when transmitting
	// safety monitored resource names in a response.
	SafetyMonitoredResourceMetadataKey = "viam-smrn"
)

type ctxKey int

const ctxKeySessionID = ctxKey(iota)

// ToContext attaches a session to the given context.
func ToContext(ctx context.Context, sess *Session) context.Context {
	return context.WithValue(ctx, ctxKeySessionID, sess)
}

// FromContext returns the current session.
func FromContext(ctx context.Context) (*Session, bool) {
	sess, ok := ctx.Value(ctxKeySessionID).(*Session)
	if !ok {
		return nil, false
	}
	return sess, true
}

// SafetyMonitor signals to the session, if present, that the given target should be
// safety monitored so that if the session ends and this session was the last
// to monitor the target, it will attempt to be stopped.
// Note: This not be called by a resource being monitored itself but instead
// by another resource or call site that is controlling a resource on behalf of
// some request/routine (e.g. a remote controller moving a base).
// In the context of a gRPC handled request, this can only be called before the
// first response is sent back (in the case of unary, before the handler returns).
func SafetyMonitor(ctx context.Context, target interface{}) {
	if target == nil {
		return
	}
	reconf, ok := target.(resource.Reconfigurable)
	if !ok {
		golog.Global().Errorf("tried to safety monitor a %T but it has no name", target)
		return
	}
	SafetyMonitorResourceName(ctx, reconf.Name())
}

// SafetyMonitorResourceName works just like SafetyMonitor but uses resource names
// directly.
func SafetyMonitorResourceName(ctx context.Context, targetName resource.Name) {
	setSafetyMonitoredResourceMetadata(ctx, targetName)
	sess, ok := FromContext(ctx)
	if !ok {
		return
	}
	// only if the session is active still
	sess.associateWith(targetName)
}

func setSafetyMonitoredResourceMetadata(ctx context.Context, name resource.Name) {
	if err := grpc.SetHeader(ctx, metadata.MD{
		SafetyMonitoredResourceMetadataKey: []string{name.String()},
	}); err != nil {
		if s, ok := status.FromError(err); !ok || s.Code() != codes.Internal {
			utils.UncheckedError(err)
		}
	}
}
