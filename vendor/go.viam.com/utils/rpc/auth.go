package rpc

import (
	"context"
	"crypto/ed25519"
	"crypto/rsa"
	//nolint:gosec // using for fingerprint
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/utils/jwks"
)

// An AuthHandler is responsible for authenticating an RPC connection. That means
// that if the idea of multiple entities can be involved in one connection, that
// this is not a suitable abstraction to use. The use of entity and subject are
// interchangeable here.
type AuthHandler interface {
	// Authenticate returns nil if the given payload is valid authentication material.
	// Optional authentication metadata can be returned to be used in future requests
	// via ContextAuthMetadata.
	Authenticate(ctx context.Context, entity, payload string) (map[string]string, error)
}

// A EntityDataLoader loads data about an entity.
type EntityDataLoader interface {
	// EntityData loads opaque info about the authenticated entity that will be bound to the
	// context accessible via ContextAuthEntity.
	EntityData(ctx context.Context, claims Claims) (interface{}, error)
}

// EntityInfo provides information about a entity specific to it's credential type's
// entity data loader.
type EntityInfo struct {
	Entity string
	Data   interface{}
}

// An AuthenticateToHandler determines if the given entity should be allowed to be
// authenticated to by the calling entity, accessible via MustContextAuthEntity.
// Similarly, the returned auth metadata will be present on the given entity's endpoints
// via ContextAuthEntity. The use of entity and subject are interchangeable here with
// respect to the entity being authenticated to.
type AuthenticateToHandler func(ctx context.Context, toEntity string) (map[string]string, error)

// TokenVerificationKeyProvider allows an auth for a cred type to supply a key needed to peform
// verification of a JWT. This is helpful when the server itself is not responsible
// for authentication. For example, this could be for a central auth server
// with untrusted peers using a public key to verify JWTs.
type TokenVerificationKeyProvider interface {
	// TokenVerificationKey returns the key needed to do JWT verification.
	TokenVerificationKey(ctx context.Context, token *jwt.Token) (interface{}, error)
	Close(ctx context.Context) error
}

// Claims is an interface that all custom claims must implement to be supported
// by the rpc system.
type Claims interface {
	// Ensure we meet the jwt.Claims interface, return error if claims are invalid. Claims
	// are validated before entity checks,
	jwt.Claims

	// Entity returns the entity associated with the claims. Also known
	// as a Subject.
	Entity() string

	// CredentialsType returns the rpc CredentialsType based on the jwt claims.
	CredentialsType() CredentialsType

	// Metadata returns the rpc auth metadata based on the jwt claims.
	Metadata() map[string]string
}

var (
	errInvalidCredentials = status.Error(codes.Unauthenticated, "invalid credentials")
	errCannotAuthEntity   = status.Error(codes.Unauthenticated, "cannot authenticate entity")
)

// AuthHandlerFunc is an AuthHandler for entities.
type AuthHandlerFunc func(ctx context.Context, entity, payload string) (map[string]string, error)

var _ AuthHandler = AuthHandlerFunc(nil)

// Authenticate checks if the given entity and payload are what it expects. It returns
// an error otherwise.
func (h AuthHandlerFunc) Authenticate(ctx context.Context, entity, payload string) (map[string]string, error) {
	return h(ctx, entity, payload)
}

// EntityDataLoaderFunc is an EntityDataLoader for entities.
type EntityDataLoaderFunc func(ctx context.Context, claims Claims) (interface{}, error)

// EntityData checks if the given entity is handled by this handler.
func (h EntityDataLoaderFunc) EntityData(ctx context.Context, claims Claims) (interface{}, error) {
	return h(ctx, claims)
}

// TokenVerificationKeyProviderFunc is a TokenVerificationKeyProvider that provides keys for
// JWT verification. Note: This function MUST do checks on the token signing method for security purposes.
type TokenVerificationKeyProviderFunc func(ctx context.Context, token *jwt.Token) (interface{}, error)

// TokenVerificationKey returns a key that can be used to verify the given token. This is used when ensuring
// an RPC request is properly authenticated.
func (p TokenVerificationKeyProviderFunc) TokenVerificationKey(ctx context.Context, token *jwt.Token) (interface{}, error) {
	return p(ctx, token)
}

// Close does nothing.
func (p TokenVerificationKeyProviderFunc) Close(ctx context.Context) error {
	return nil
}

// MakeRSAPublicKeyProvider returns a TokenVerificationKeyProvider that provides a public key for JWT verification.
func MakeRSAPublicKeyProvider(pubKey *rsa.PublicKey) TokenVerificationKeyProvider {
	return TokenVerificationKeyProviderFunc(
		func(ctx context.Context, token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
			}

			return pubKey, nil
		},
	)
}

// MakeEd25519PublicKeyProvider returns a TokenVerificationKeyProvider that provides a public key for JWT verification.
func MakeEd25519PublicKeyProvider(pubKey ed25519.PublicKey) TokenVerificationKeyProvider {
	return TokenVerificationKeyProviderFunc(
		func(ctx context.Context, token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
				return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
			}

			return pubKey, nil
		},
	)
}

// MakeOIDCKeyProvider returns a TokenVerificationKeyProvider that dynamically looks up a public key for
// JWT verification by inspecting the JWT's kid field. The given issuer is used to discover the JWKs
// used for verification. This issuer is expected to follow the OIDC Discovery protocol.
func MakeOIDCKeyProvider(ctx context.Context, issuer string) (TokenVerificationKeyProvider, error) {
	provider, err := jwks.NewCachingOIDCJWKKeyProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	return MakeJWKSKeyProvider(provider), nil
}

// MakeJWKSKeyProvider returns a TokenVerificationKeyProvider that dynamically looks up a public key for
// JWT verification by inspecting the JWT's kid field. The given JWK key provider is used to look up
// keys from the JWT.
func MakeJWKSKeyProvider(provider jwks.KeyProvider) TokenVerificationKeyProvider {
	return &oidcKeyProvider{jwkProvider: provider}
}

type oidcKeyProvider struct {
	jwkProvider jwks.KeyProvider
}

// TokenVerificationKey returns the public key needed to do JWT verification by inspecting
// the JWT's kid field and asking the provider to find the corresponding key.
func (op *oidcKeyProvider) TokenVerificationKey(ctx context.Context, token *jwt.Token) (ret interface{}, err error) {
	keyID, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errors.New("kid header not in token header")
	}

	return op.jwkProvider.LookupKey(ctx, keyID, token.Method.Alg())
}

// Close closes the jwks key provider.
func (op *oidcKeyProvider) Close(ctx context.Context) error {
	return op.jwkProvider.Close()
}

// MakeSimpleAuthHandler returns a simple auth handler that handles multiple entities
// sharing one payload. This is useful for setting up local/internal authentication with a
// shared key. This is NOT secure for usage over networks exposed to the public internet.
// For that, use a more sophisticated handler with at least one key per entity.
func MakeSimpleAuthHandler(forEntities []string, expectedPayload string) AuthHandler {
	return MakeSimpleMultiAuthHandler(forEntities, []string{expectedPayload})
}

// MakeSimpleMultiAuthHandler returns a simple auth handler that handles multiple entities
// sharing multiple possible payloads. This is useful for rolling keys.
func MakeSimpleMultiAuthHandler(forEntities, expectedPayloads []string) AuthHandler {
	if len(expectedPayloads) == 0 {
		panic("expected at least one payload")
	}
	entityChecker := MakeEntitiesChecker(forEntities)
	return AuthHandlerFunc(func(ctx context.Context, entity, payload string) (map[string]string, error) {
		if err := entityChecker(ctx, entity); err != nil {
			if errors.Is(err, errCannotAuthEntity) {
				return nil, errInvalidCredentials
			}
			return nil, err
		}

		payloadB := []byte(payload)
		for _, expectedPayload := range expectedPayloads {
			if subtle.ConstantTimeCompare(payloadB, []byte(expectedPayload)) == 1 {
				return map[string]string{}, nil
			}
		}
		return nil, errInvalidCredentials
	})
}

// MakeSimpleMultiAuthPairHandler works similarly to MakeSimpleMultiAuthHandler with the addition of
// supporting a key, id pair used to ensure that a key that maps to the id matches the key passed
// during the function call.
func MakeSimpleMultiAuthPairHandler(expectedPayloads map[string]string) AuthHandler {
	if len(expectedPayloads) == 0 {
		panic("expected at least one payload")
	}

	return AuthHandlerFunc(func(ctx context.Context, entity, payload string) (map[string]string, error) {
		if _, ok := expectedPayloads[entity]; !ok {
			return nil, errInvalidCredentials
		}

		if subtle.ConstantTimeCompare([]byte(expectedPayloads[entity]), []byte(payload)) == 1 {
			return map[string]string{}, nil
		}
		return nil, errInvalidCredentials
	})
}

// MakeEntitiesChecker checks a list of entities against a given one for use in an auth handler.
func MakeEntitiesChecker(forEntities []string) func(ctx context.Context, entities ...string) error {
	return func(ctx context.Context, entities ...string) error {
		for _, recvEntity := range entities {
			for _, checkEntity := range forEntities {
				if subtle.ConstantTimeCompare([]byte(recvEntity), []byte(checkEntity)) == 1 {
					return nil
				}
			}
		}
		return errCannotAuthEntity
	}
}

// CredentialsType signifies a means of representing a credential. For example,
// an API key.
type CredentialsType string

const (
	credentialsTypeInternal = CredentialsType("__internal")
	// CredentialsTypeAPIKey is intended for by external users, human and computer.
	CredentialsTypeAPIKey = CredentialsType("api-key")

	// CredentialsTypeExternal is for credentials that are to be produced by some
	// external authentication endpoint (see ExternalAuthService#AuthenticateTo) intended
	// for another, different consumer at a different endpoint.
	CredentialsTypeExternal = CredentialsType("external")
)

// Credentials packages up both a type of credential along with its payload which
// is formatted specific to the type.
type Credentials struct {
	Type    CredentialsType `json:"type"`
	Payload string          `json:"payload"`
}

type credAuthHandlers struct {
	AuthHandler                  AuthHandler
	EntityDataLoader             EntityDataLoader
	TokenVerificationKeyProvider TokenVerificationKeyProvider
}

// RSAPublicKeyThumbprint returns SHA1 of the public key's modulus Base64 URL encoded without padding.
func RSAPublicKeyThumbprint(key *rsa.PublicKey) (string, error) {
	//nolint:gosec // using for fingerprint
	thumbPrint := sha1.New()
	_, err := thumbPrint.Write(key.N.Bytes())
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(thumbPrint.Sum(nil)), nil
}

// ED25519PublicKeyThumbprint returns the base64 encoded public key.
func ED25519PublicKeyThumbprint(key ed25519.PublicKey) string {
	return base64.RawURLEncoding.EncodeToString(key)
}
