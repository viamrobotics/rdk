package grpc

import (
	"context"
	"fmt"

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

// String renders the identity for human-readable logs, e.g.
// "steve@viam.com (user_id 11111)". It prefers e-mail and app user ID when present and
// falls back to the raw entity or an "unauthenticated client" note.
func (id Identity) String() string {
	if !id.Authenticated() {
		return "unauthenticated client"
	}
	switch {
	case id.Email != "" && id.AppUserID != "":
		return fmt.Sprintf("%s (user_id %s)", id.Email, id.AppUserID)
	case id.Email != "":
		return id.Email
	case id.AppUserID != "":
		return fmt.Sprintf("user_id %s", id.AppUserID)
	default:
		return id.Entity
	}
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
