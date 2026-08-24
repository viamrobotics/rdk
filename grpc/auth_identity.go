package grpc

import (
	"context"

	"go.viam.com/utils/rpc"
)

const (
	// appUserIDClaim is the auth metadata claim carrying the caller's Viam app user ID.
	appUserIDClaim = "app_user_id"
	// emailClaim is the auth metadata claim carrying the caller's e-mail. It is used only
	// for human-readable logging, never for authorization (e-mails are PII and mutable).
	emailClaim = "email"
)

// Identity is the authorization-relevant identity of a connected client.
type Identity struct {
	// Entity is the JWT subject: an API key ID or a FusionAuth user ID. Empty if the
	// client is unauthenticated.
	Entity string
	// AppUserID is the Viam app user ID from the token's auth metadata claim, if any.
	AppUserID string
	// Email is the caller's e-mail from the token's auth metadata claim, if any. It is
	// carried for logging only and must never be used as part of the authorization key
	// (see permsFor); e-mails are PII and can change.
	Email string
}

// Authenticated returns whether the identity belongs to an authenticated client.
func (id Identity) Authenticated() bool {
	return id.Entity != ""
}

// AuthType returns the kind of credential the caller was identified by, for logging:
// "app-user-id" when the app_user_id claim is present, otherwise "api-key-id" (the JWT
// subject, which is an API key ID for API-key auth), or "unauthenticated".
func (id Identity) AuthType() string {
	switch {
	case !id.Authenticated():
		return "unauthenticated"
	case id.AppUserID != "":
		return "app-user-id"
	default:
		return "api-key-id"
	}
}

// AuthID returns the identifying value matching AuthType: the app user ID for an
// app-user-id caller, otherwise the JWT subject (API key ID).
func (id Identity) AuthID() string {
	if id.AppUserID != "" {
		return id.AppUserID
	}
	return id.Entity
}

// IdentityFromContext extracts the caller's identity from a request context. The
// returned identity is zero-valued for unauthenticated clients. The entity, app user
// ID, and e-mail all come from the auth entity that the auth layer (or, over WebRTC,
// the signaling-forwarded token) placed on the context; no token re-parsing is needed.
func IdentityFromContext(ctx context.Context) Identity {
	entity, ok := rpc.ContextAuthEntity(ctx)
	if !ok {
		return Identity{}
	}
	return Identity{
		Entity:    entity.Entity,
		AppUserID: entity.AuthMetadata[appUserIDClaim],
		Email:     entity.AuthMetadata[emailClaim],
	}
}
