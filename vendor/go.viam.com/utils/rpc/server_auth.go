package rpc

import (
	"context"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	rpcpb "go.viam.com/utils/proto/rpc/v1"
)

func (ss *simpleServer) authHandlers(forType CredentialsType) (credAuthHandlers, error) {
	handler, ok := ss.authHandlersForCreds[forType]
	if !ok {
		return credAuthHandlers{}, status.Errorf(codes.InvalidArgument, "do not know how to handle credential type %q", forType)
	}
	return handler, nil
}

const (
	// MetadataFieldAuthorization is a constant for the authorization header key.
	MetadataFieldAuthorization = "authorization"
	// AuthorizationValuePrefixBearer is a constant for the Bearer token prefix.
	AuthorizationValuePrefixBearer = "Bearer "
)

// JWTClaims extends jwt.RegisteredClaims with information about the credentials as well
// as authentication metadata.
type JWTClaims struct {
	jwt.RegisteredClaims
	AuthCredentialsType CredentialsType   `json:"rpc_creds_type,omitempty"`
	AuthMetadata        map[string]string `json:"rpc_auth_md,omitempty"`
	ApplicationID       string            `json:"applicationId,omitempty"`
}

// Entity returns the entity from the claims' Subject.
func (c JWTClaims) Entity() string {
	return c.RegisteredClaims.Subject
}

// CredentialsType returns the credential type from `rpc_creds_type` claim.
func (c JWTClaims) CredentialsType() CredentialsType {
	return c.AuthCredentialsType
}

// Metadata returns the metadata from `rpc_auth_md` claim.
func (c JWTClaims) Metadata() map[string]string {
	if len(c.AuthMetadata) == 0 {
		return nil
	}
	mdClone := make(map[string]string, len(c.AuthMetadata))
	for key, value := range c.AuthMetadata {
		mdClone[key] = value
	}
	return mdClone
}

// ensure JWTClaims implements Claims.
var _ Claims = JWTClaims{}

func (ss *simpleServer) Authenticate(ctx context.Context, req *rpcpb.AuthenticateRequest) (*rpcpb.AuthenticateResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("expected metadata")
	}
	if len(md[MetadataFieldAuthorization]) != 0 {
		return nil, status.Error(codes.InvalidArgument, "already authenticated; cannot re-authenticate")
	}
	if req.GetCredentials() == nil {
		return nil, status.Error(codes.InvalidArgument, "credentials required")
	}
	forType := CredentialsType(req.GetCredentials().GetType())
	handlers, err := ss.authHandlers(forType)
	if err != nil {
		return nil, err
	}
	if handlers.AuthHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "direct authentication not supported for %q", forType)
	}
	authMD, err := handlers.AuthHandler.Authenticate(ctx, req.GetEntity(), req.GetCredentials().GetPayload())
	if err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		return nil, status.Errorf(codes.PermissionDenied, "failed to authenticate: %s", err.Error())
	}

	// We sign tokens destined for ourselves. If they are not for ourselves but for the entity, then
	// AuthenticateTo should be used.
	token, err := ss.signAccessTokenForEntity(forType, ss.authAudience, req.GetEntity(), authMD)
	if err != nil {
		return nil, err
	}

	return &rpcpb.AuthenticateResponse{
		AccessToken: token,
	}, nil
}

func (ss *simpleServer) AuthenticateTo(ctx context.Context, req *rpcpb.AuthenticateToRequest) (*rpcpb.AuthenticateToResponse, error) {
	// Use the entity from the original authenticated call/payload.
	entity, ok := ContextAuthEntity(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "entity should be available")
	}

	authMD, err := ss.authToHandler(ctx, req.GetEntity())
	if err != nil {
		return nil, err
	}

	token, err := ss.signAccessTokenForEntity(CredentialsTypeExternal, []string{req.GetEntity()}, entity.Entity, authMD)
	if err != nil {
		return nil, err
	}

	return &rpcpb.AuthenticateToResponse{
		AccessToken: token,
	}, nil
}

func (ss *simpleServer) signAccessTokenForEntity(
	forType CredentialsType,
	audience []string,
	entity string,
	authMD map[string]string,
) (string, error) {
	// TODO(GOUT-13): expiration
	// TODO(GOUT-12): refresh token
	// TODO(GOUT-9): more complete info
	token := jwt.NewWithClaims(ss.authKeyForJWTSigning.method, JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  entity,
			Audience: audience,
			Issuer:   ss.authIssuer,
			IssuedAt: jwt.NewNumericDate(time.Now()),
			ID:       uuid.NewString(),
		},
		AuthCredentialsType: forType,
		AuthMetadata:        authMD,
	})

	// Set the Key ID (kid) to allow the auth handlers to selectively choose which key was used
	// to sign the token.
	token.Header["kid"] = ss.authKeyForJWTSigning.id

	tokenString, err := token.SignedString(ss.authKeyForJWTSigning.privateKey)
	if err != nil {
		ss.logger.Errorw("failed to sign JWT", "error", err)
		return "", status.Error(codes.PermissionDenied, "failed to authenticate")
	}

	return tokenString, nil
}

func (ss *simpleServer) isPublicMethod(
	fullMethod string,
) bool {
	return ss.publicMethods[fullMethod]
}

func (ss *simpleServer) authUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// no auth
	if ss.exemptMethods[info.FullMethod] {
		return handler(ctx, req)
	}

	// optional auth
	if ss.isPublicMethod(info.FullMethod) {
		nextCtx, err := ss.tryAuth(ctx)
		if err != nil {
			return nil, err
		}
		return handler(nextCtx, req)
	}

	// private auth
	nextCtx, err := ss.ensureAuthed(ctx)
	if err != nil {
		return nil, err
	}

	return handler(nextCtx, req)
}

func (ss *simpleServer) authStreamInterceptor(
	srv interface{},
	serverStream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	if ss.exemptMethods[info.FullMethod] {
		return handler(srv, serverStream)
	}

	// optional auth
	if ss.isPublicMethod(info.FullMethod) {
		nextCtx, err := ss.tryAuth(serverStream.Context())
		if err != nil {
			return err
		}
		serverStream = ctxWrappedServerStream{serverStream, nextCtx}
		return handler(srv, serverStream)
	}

	// private auth
	nextCtx, err := ss.ensureAuthed(serverStream.Context())
	if err != nil {
		return err
	}

	serverStream = ctxWrappedServerStream{serverStream, nextCtx}
	return handler(srv, serverStream)
}

type ctxWrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (wrapped ctxWrappedServerStream) Context() context.Context {
	return wrapped.ctx
}

// TokenFromContext returns the bearer token from the authorization header and errors if it does not exist.
func TokenFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authentication required")
	}
	authHeader := md.Get(MetadataFieldAuthorization)
	if len(authHeader) != 1 {
		return "", status.Error(codes.Unauthenticated, "authentication required")
	}
	if !strings.HasPrefix(authHeader[0], AuthorizationValuePrefixBearer) {
		return "", status.Errorf(codes.Unauthenticated, "expected authorization header with prefix: %s", AuthorizationValuePrefixBearer)
	}
	return strings.TrimPrefix(authHeader[0], AuthorizationValuePrefixBearer), nil
}

var errNotTLSAuthed = errors.New("not authenticated via TLS")

var validSigningMethods = []string{
	"ES256",
	"ES512",
	"HS512",
	"PS256",
	"RS512",
	"PS384",
	"PS512",
	"RS384",
	"ES384",
	"EdDSA",
	"HS256",
	"HS384",
	"RS256",
}

// tryAuth is called for public methods where auth is not required but preferable.
func (ss *simpleServer) tryAuth(ctx context.Context) (context.Context, error) {
	nextCtx, err := ss.ensureAuthed(ctx)
	if err != nil {
		if status, _ := status.FromError(err); status.Code() != codes.Unauthenticated {
			return nil, err
		}
		return ctx, nil
	}
	return nextCtx, nil
}

func (ss *simpleServer) ensureAuthed(ctx context.Context) (context.Context, error) {
	// Use handler if set (only used for testing).
	if ss.ensureAuthedHandler != nil {
		return ss.ensureAuthedHandler(ctx)
	}

	tokenString, err := TokenFromContext(ctx)
	if err != nil {
		// check TLS state
		if ss.tlsAuthHandler == nil {
			return nil, err
		}
		var verifiedCert *x509.Certificate
		if p, ok := peer.FromContext(ctx); ok && p.AuthInfo != nil {
			if authInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
				verifiedChains := authInfo.State.VerifiedChains
				if len(verifiedChains) != 0 && len(verifiedChains[0]) != 0 {
					verifiedCert = verifiedChains[0][0]
				}
			}
		}
		if verifiedCert == nil {
			return nil, err
		}
		if tlsErr := ss.tlsAuthHandler(ctx, verifiedCert.DNSNames...); tlsErr == nil {
			// mTLS based authentication contexts do not really have a sense of a unique identifier
			// when considering multiple clients using the certificate. We deem this okay but it does
			// mean that if the identifier is used to bind to the concept of a unique session, it is
			// not sufficient without another piece of information (like an address and port).
			// Furthermore, if TLS certificate verification is disabled, this trust is lost.
			// Our best chance at uniqueness with a compliant CA is to use the issuer DN (Distinguished Name)
			// along with the serial number; compliancy hinges on issuing unique serial numbers and if this
			// is an intermediate CA, their parent issuing unique DNs.
			nextCtx := ContextWithAuthEntity(ctx, EntityInfo{
				Entity: verifiedCert.Issuer.String() + ":" + verifiedCert.SerialNumber.String(),
			})
			return nextCtx, nil
		} else if !errors.Is(tlsErr, errNotTLSAuthed) {
			return nil, multierr.Combine(err, tlsErr)
		}
		return nil, err
	}

	var claims JWTClaims
	var handlers credAuthHandlers
	if _, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (interface{}, error) {
			var err error
			handlers, err = ss.authHandlers(claims.CredentialsType())
			if err != nil {
				return nil, err
			}

			if handlers.TokenVerificationKeyProvider != nil {
				return handlers.TokenVerificationKeyProvider.TokenVerificationKey(ctx, token)
			}

			// signed by us, so we always have a kid
			keyID, ok := token.Header["kid"].(string)
			if !ok {
				return nil, errors.New("kid header not in token header")
			}

			keyData, ok := ss.authKeys[keyID]
			if !ok {
				return nil, fmt.Errorf("this server did not sign this JWT with kid %q", keyID)
			}

			if keyData.method == nil {
				ss.logger.Errorw("invariant: auth key data has no method", "kid", keyID)
				return "", errors.New("internal server error")
			}
			if token.Method != keyData.method {
				return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
			}

			return keyData.publicKey, nil
		},
		jwt.WithValidMethods(validSigningMethods),
	); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthenticated: %s", err)
	}

	// Audience verification is critical for security. Without it, we have a higher chance
	// of validating a JWT is valid, but not that it is intended for us. Of course, that means
	// we trust whomever owns the private keys to signing access tokens.
	audVerified := false
	for _, allowdAud := range ss.authAudience {
		if claims.RegisteredClaims.VerifyAudience(allowdAud, true) {
			audVerified = true
			break
		}
	}

	if !audVerified {
		var claimAudience []byte
		err := claims.RegisteredClaims.Audience.UnmarshalJSON(claimAudience)
		if err != nil {
			ss.logger.Errorw("invalid audience: cannot unmarshall audience claim",
				"error", err,
				"registered_claims", claims.RegisteredClaims,
				"authAudience", ss.authAudience)
			return nil, status.Error(codes.Unauthenticated, "invalid audience")
		}
		ss.logger.Errorw("invalid audience",
			"registered_claims", claims.RegisteredClaims,
			"authAudience", ss.authAudience)
		return nil, status.Error(codes.Unauthenticated,
			"invalid audience (registered aud claim: "+string(claimAudience)+")")
	}

	// Note(erd): may want to verify issuers in the future where the claims/scope are
	// treated differently if it comes down to permissions encoded in a JWT.

	err = claims.Valid()
	if err != nil {
		ss.logger.Errorw("invalid claims",
			"error", err,
			"registered_claims", claims.RegisteredClaims)
		return nil, status.Errorf(codes.Unauthenticated, "unauthenticated: %s", err)
	}

	claimsEntity := claims.Entity()
	if claimsEntity == "" {
		ss.logger.Errorw("invalid claims entity: expected entity (sub) in claims",
			"error", err,
			"registered_claims", claims.RegisteredClaims)
		return nil, status.Errorf(codes.Unauthenticated, "expected entity (sub) in claims")
	}

	var entityData interface{}
	if handlers.EntityDataLoader != nil {
		data, err := handlers.EntityDataLoader.EntityData(ctx, claims)
		if err != nil {
			if _, ok := status.FromError(err); ok {
				return nil, err
			}
			return nil, status.Errorf(codes.Internal, "failed to load entity data: %s", err)
		}
		entityData = data
	}

	return ContextWithAuthEntity(ctx, EntityInfo{claimsEntity, entityData}), nil
}
