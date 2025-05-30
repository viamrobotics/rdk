package rpc

import (
	"context"
	"errors"

	"github.com/viamrobotics/webrtc/v3"
)

type ctxKey int

const (
	ctxKeyHost = ctxKey(iota)
	ctxKeyDialer
	ctxKeyPeerConnection
	ctxKeyAuthEntity
	ctxKeyAuthClaims // all jwt claims
)

// contextWithHost attaches a host name to the given context.
func contextWithHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, ctxKeyHost, host)
}

// contextHost returns a host name. It may be nil if the value was never set.
func contextHost(ctx context.Context) string {
	host := ctx.Value(ctxKeyHost)
	if host == nil {
		return ""
	}
	return host.(string)
}

// ContextWithDialer attaches a Dialer to the given context.
func ContextWithDialer(ctx context.Context, d Dialer) context.Context {
	return context.WithValue(ctx, ctxKeyDialer, d)
}

// contextDialer returns a Dialer. It may be nil if the value was never set.
func contextDialer(ctx context.Context) Dialer {
	dialer := ctx.Value(ctxKeyDialer)
	if dialer == nil {
		return nil
	}
	return dialer.(Dialer)
}

// ContextWithPeerConnection attaches a peer connection to the given context.
func ContextWithPeerConnection(ctx context.Context, pc *webrtc.PeerConnection) context.Context {
	return context.WithValue(ctx, ctxKeyPeerConnection, pc)
}

// ContextPeerConnection returns a peer connection, if set.
func ContextPeerConnection(ctx context.Context) (*webrtc.PeerConnection, bool) {
	pc := ctx.Value(ctxKeyPeerConnection)
	if pc == nil {
		return nil, false
	}
	return pc.(*webrtc.PeerConnection), true
}

// ContextWithAuthEntity attaches an entity (e.g. a user) for an authenticated context to the given context.
func ContextWithAuthEntity(ctx context.Context, authEntity EntityInfo) context.Context {
	return context.WithValue(ctx, ctxKeyAuthEntity, authEntity)
}

// ContextAuthEntity returns the entity (e.g. a user) associated with this authentication context.
func ContextAuthEntity(ctx context.Context) (EntityInfo, bool) {
	authEntityValue := ctx.Value(ctxKeyAuthEntity)
	if authEntityValue == nil {
		return EntityInfo{}, false
	}
	authEntity, ok := authEntityValue.(EntityInfo)
	if !ok || authEntity.Entity == "" {
		return EntityInfo{}, false
	}
	return authEntity, true
}

// MustContextAuthEntity returns the entity associated with this authentication context;
// it panics if there is none set.
func MustContextAuthEntity(ctx context.Context) EntityInfo {
	authEntity, has := ContextAuthEntity(ctx)
	if !has {
		panic(errors.New("no auth entity present"))
	}
	return authEntity
}
