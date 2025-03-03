package grpc

import (
	"context"
	"sync"
	"time"

	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// DefaultMethodTimeout is the default context timeout for all inbound gRPC
// methods and all outbound gRPC methods to modules, only used when no
// deadline is set on the context.
var DefaultMethodTimeout = 10 * time.Minute

// EnsureTimeoutUnaryServerInterceptor sets a default timeout on the context if one is
// not already set. To be called as the first unary server interceptor.
func EnsureTimeoutUnaryServerInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (interface{}, error) {
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultMethodTimeout)
		defer cancel()
	}

	return handler(ctx, req)
}

// EnsureTimeoutUnaryClientInterceptor sets a default timeout on the context if one is
// not already set. To be called as the first unary client interceptor.
func EnsureTimeoutUnaryClientInterceptor(
	ctx context.Context,
	method string, req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultMethodTimeout)
		defer cancel()
	}

	return invoker(ctx, method, req, reply, cc, opts...)
}

// The following code is for appending/extracting grpc metadata regarding module names/origins via
// contexts.
type modNameKeyType int

const modNameKeyID = modNameKeyType(iota)

// GetModuleName returns the module name (if any) the request came from. The module name will match
// a string from the robot config.
func GetModuleName(ctx context.Context) string {
	valI := ctx.Value(modNameKeyID)
	if val, ok := valI.(string); ok {
		return val
	}

	return ""
}

const modNameMetadataKey = "modName"

// ModInterceptors takes a user input `ModName` and exposes an interceptor method that will attach
// it to outgoing gRPC requests.
type ModInterceptors struct {
	ModName string
}

// UnaryClientInterceptor adds a module name to any outgoing unary gRPC request.
func (mc *ModInterceptors) UnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	ctx = metadata.AppendToOutgoingContext(ctx, modNameMetadataKey, mc.ModName)
	return invoker(ctx, method, req, reply, cc, opts...)
}

// ModPeerConnTracker is an object (owned by the web service) that manages Module <-> PeerConnection
// mappings. And provides interceptors for attaching module name and PeerConnection objects onto
// context's for unary calls.
type ModPeerConnTracker struct {
	mu                sync.Mutex
	modNameToPeerConn map[string]*webrtc.PeerConnection
}

// NewModPeerConnTracker creates a new ModPeerConnTracker.
func NewModPeerConnTracker() *ModPeerConnTracker {
	return &ModPeerConnTracker{
		modNameToPeerConn: make(map[string]*webrtc.PeerConnection),
	}
}

// Add informs the ModPeerConnTracker of a new module name <-> PeerConnection mapping.
func (tracker *ModPeerConnTracker) Add(modname string, peerConn *webrtc.PeerConnection) {
	tracker.mu.Lock()
	tracker.modNameToPeerConn[modname] = peerConn
	tracker.mu.Unlock()
}

// Remove removes a mapping from the tracker.
func (tracker *ModPeerConnTracker) Remove(modname string) {
	tracker.mu.Lock()
	delete(tracker.modNameToPeerConn, modname)
	tracker.mu.Unlock()
}

// ModInfoUnaryServerInterceptor checks the incoming RPC metadata for a module name and attaches any
// information to a context that can be retrieved with `GetModuleName`.
func (tracker *ModPeerConnTracker) ModInfoUnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return handler(ctx, req)
	}

	values := meta.Get(modNameMetadataKey)
	if len(values) == 1 {
		// We have a module name. Attach it for anyone interested.
		modName := values[0]
		ctx = context.WithValue(ctx, modNameKeyID, values[0])

		tracker.mu.Lock()
		pc, exists := tracker.modNameToPeerConn[modName]
		tracker.mu.Unlock()

		if exists {
			// We also have that module mapped to a PeerConnection. Attach that as well.
			ctx = rpc.ContextWithPeerConnection(ctx, pc)
		}
	}

	return handler(ctx, req)
}
