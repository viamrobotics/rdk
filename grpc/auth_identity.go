package grpc

import (
	"context"

	jwt "github.com/golang-jwt/jwt/v4"
	"go.viam.com/utils/rpc"
)

// Identity is the authorization-relevant identity of a connected client.
type Identity struct {
	// Entity is the JWT subject: an API key ID or a FusionAuth user ID. Empty if the
	// client is unauthenticated.
	Entity string
	// Email is the e-mail from the token's auth metadata claim, if any.
	Email string
}

// Authenticated returns whether the identity belongs to an authenticated client.
func (id Identity) Authenticated() bool {
	return id.Entity != ""
}

// IdentityFromContext extracts the caller's identity from a request context. The
// returned identity is zero-valued for unauthenticated clients.
func IdentityFromContext(ctx context.Context) Identity {
	entity, ok := rpc.ContextAuthEntity(ctx)
	if !ok {
		return Identity{}
	}
	var email string
	token, err := rpc.TokenFromContext(ctx)
	if err == nil {
		// The token's signature was already verified by the auth interceptor, so parsing it
		// unverified here is safe.
		var claims rpc.JWTClaims
		if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err == nil {
			email = claims.Metadata()["email"]
		}
	}

	return Identity{Entity: entity.Entity, Email: email}
}
