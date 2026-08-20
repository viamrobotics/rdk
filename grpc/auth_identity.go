package grpc

import (
	"context"

	"go.viam.com/utils/rpc"
)

// appUserIDClaim is the auth metadata claim carrying the caller's Viam app user ID.
const appUserIDClaim = "app_user_id"

// Identity is the authorization-relevant identity of a connected client.
type Identity struct {
	// Entity is the JWT subject: an API key ID or a FusionAuth user ID. Empty if the
	// client is unauthenticated.
	Entity string
	// AppUserID is the Viam app user ID from the token's auth metadata claim, if any.
	AppUserID string
}

// Authenticated returns whether the identity belongs to an authenticated client.
func (id Identity) Authenticated() bool {
	return id.Entity != ""
}

// IdentityFromContext extracts the caller's identity from a request context. The
// returned identity is zero-valued for unauthenticated clients. Both the entity and
// the app user ID come from the auth entity that the auth layer (or, over WebRTC, the
// signaling-forwarded token) placed on the context; no token re-parsing is needed.
func IdentityFromContext(ctx context.Context) Identity {
	entity, ok := rpc.ContextAuthEntity(ctx)
	if !ok {
		return Identity{}
	}
	return Identity{Entity: entity.Entity, AppUserID: entity.AuthMetadata[appUserIDClaim]}
}
