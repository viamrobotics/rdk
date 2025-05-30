package rpc

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
	"github.com/viamrobotics/webrtc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"

	"go.viam.com/utils/jwks"
)

// serverOptions change the runtime behavior of the server.
type serverOptions struct {
	bindAddress       string
	listenerAddress   *net.TCPAddr
	tlsConfig         *tls.Config
	webrtcOpts        WebRTCServerOptions
	unaryInterceptor  grpc.UnaryServerInterceptor
	streamInterceptor grpc.StreamServerInterceptor

	// instanceNames are the name of this server and will be used
	// to report itself over mDNS.
	instanceNames []string

	// unauthenticated determines if requests should be authenticated.
	unauthenticated bool

	// allowUnauthenticatedHealthCheck allows the server to have an unauthenticated healthcheck endpoint
	allowUnauthenticatedHealthCheck bool

	// publicMethods are api routes that attempt, but do not require, authentication
	publicMethods []string

	// authKeys are used to sign/verify JWTs for authentication
	authKeys       map[string]authKeyData
	jwtSignerKeyID string

	// debug is helpful to turn on when the library isn't working quite right.
	// It will output much more logs.
	debug bool

	tlsAuthHandler       func(ctx context.Context, entities ...string) error
	authHandlersForCreds map[CredentialsType]credAuthHandlers

	// authAudience is the JWT audience (aud) that will be used/expected
	// for our service. When unset, it will be debug logged that
	// the instance names will be used instead.
	authAudience []string

	// authIssuer is the JWT issuer (iss) that will be used for our service.
	// When unset, it will be debug logged that the first audience member will
	// be used instead.
	authIssuer string

	authToHandler AuthenticateToHandler
	disableMDNS   bool

	// stats monitoring on the connections.
	statsHandler stats.Handler

	// ensureAuthedHandler is the callback used to ensure that the context of an
	// incoming RPC request is properly authenticated.
	ensureAuthedHandler func(ctx context.Context) (context.Context, error)

	unknownStreamDesc *grpc.StreamDesc
}

type authKeyData struct {
	id         string
	method     jwt.SigningMethod
	privateKey crypto.Signer
	publicKey  interface{}
}

func (d *authKeyData) Validate() error {
	if d.id == "" {
		return errors.New("invariant: auth key has no id")
	}
	if d.method == nil {
		return fmt.Errorf("invariant: auth key %q has no signing method", d.id)
	}
	if d.privateKey == nil {
		return fmt.Errorf("invariant: auth key %q has no private key", d.id)
	}
	if d.publicKey == nil {
		return fmt.Errorf("invariant: auth key %q has no public key", d.id)
	}
	return nil
}

// WebRTCServerOptions control how WebRTC is utilized in a server.
type WebRTCServerOptions struct {
	// Enable controls if WebRTC should be turned on. It is disabled
	// by default since signaling has the potential to open up random
	// ports on the host which may not be expected.
	Enable bool

	// ExternalSignalingDialOpts are the options used to dial the external signaler.
	ExternalSignalingDialOpts []DialOption

	// ExternalSignalingAddress specifies where the WebRTC signaling
	// answerer should connect to and "listen" from. If it is empty,
	// it will connect to the server's internal address acting as
	// an answerer for itself.
	ExternalSignalingAddress string

	// EnableInternalSignaling specifies whether an internal signaling answerer
	// should be started up. This is useful if you want to have a fallback
	// server if the external cannot be reached. It is started up by default
	// if ExternalSignalingAddress is unset.
	EnableInternalSignaling bool

	// ExternalSignalingHosts specifies what hosts are being listened for when answering
	// externally.
	ExternalSignalingHosts []string

	// InternalSignalingHosts specifies what hosts are being listened for when answering
	// internally.
	InternalSignalingHosts []string

	// Config is the WebRTC specific configuration (i.e. ICE settings)
	Config *webrtc.Configuration
}

// A ServerOption changes the runtime behavior of the server.
// Cribbed from https://github.com/grpc/grpc-go/blob/aff571cc86e6e7e740130dbbb32a9741558db805/dialoptions.go#L41
type ServerOption interface {
	apply(*serverOptions) error
}

// funcServerOption wraps a function that modifies serverOptions into an
// implementation of the ServerOption interface.
type funcServerOption struct {
	f func(*serverOptions) error
}

func (fdo *funcServerOption) apply(do *serverOptions) error {
	return fdo.f(do)
}

func newFuncServerOption(f func(*serverOptions) error) *funcServerOption {
	return &funcServerOption{
		f: f,
	}
}

// WithInternalBindAddress returns a ServerOption which sets the bind address
// for the gRPC listener. If unset, the address is localhost on a
// random port unless TLS is turned on and authentication is enabled
// in which case the server will bind to all interfaces.
func WithInternalBindAddress(address string) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.bindAddress = address
		return nil
	})
}

// WithExternalListenerAddress returns a ServerOption which sets the listener address
// if the server is going to be served via its handlers and not internally.
// This is only helpful for mDNS broadcasting. If the server has TLS enabled
// internally (see WithInternalTLSConfig), then the internal listener will
// bind everywhere and this option may not be needed.
func WithExternalListenerAddress(address *net.TCPAddr) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.listenerAddress = address
		return nil
	})
}

// WithInternalTLSConfig returns a ServerOption which sets the TLS config
// for the internal listener. This is needed to have mutual TLS authentication
// work (see WithTLSAuthHandler). When using ServeTLS on the server, which serves
// from an external listener, with mutual TLS authentication, you will want to pass
// its own tls.Config with ClientAuth, at a minimum, set to tls.VerifyClientCertIfGiven.
func WithInternalTLSConfig(config *tls.Config) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.tlsConfig = config.Clone()
		if o.tlsConfig.ClientAuth == 0 {
			o.tlsConfig.ClientAuth = tls.VersionTLS12
		}
		o.tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
		return nil
	})
}

// WithWebRTCServerOptions returns a ServerOption which sets the WebRTC options
// to use if the server sets up serving WebRTC connections.
func WithWebRTCServerOptions(webrtcOpts WebRTCServerOptions) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.webrtcOpts = webrtcOpts
		return nil
	})
}

// WithUnaryServerInterceptor returns a ServerOption that sets a interceptor for
// all unary grpc methods registered. It will run after authentication and prior
// to the registered method.
func WithUnaryServerInterceptor(unaryInterceptor grpc.UnaryServerInterceptor) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.unaryInterceptor = unaryInterceptor
		return nil
	})
}

// WithStreamServerInterceptor returns a ServerOption that sets a interceptor for
// all stream grpc methods registered. It will run after authentication and prior
// to the registered method.
func WithStreamServerInterceptor(streamInterceptor grpc.StreamServerInterceptor) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.streamInterceptor = streamInterceptor
		return nil
	})
}

// WithInstanceNames returns a ServerOption which sets the names for this
// server instance. These names will be used for auth token issuance (first name) and
// mDNS service discovery to report the server itself. If unset the value
// is the address set by WithExternalListenerAddress, WithInternalBindAddress,
// or the localhost and random port address, in preference order from left to right.
func WithInstanceNames(names ...string) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		if len(names) == 0 {
			return errors.New("expected at least one instance name")
		}
		o.instanceNames = names
		return nil
	})
}

// WithUnauthenticated returns a ServerOption which turns off all authentication
// to the server's endpoints.
func WithUnauthenticated() ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.unauthenticated = true
		return nil
	})
}

// WithAuthRSAPrivateKey returns a ServerOption which sets the RSA private key to
// use for signed JWTs.
func WithAuthRSAPrivateKey(authRSAPrivateKey *rsa.PrivateKey) (ServerOption, string, error) {
	thumbprint, err := RSAPublicKeyThumbprint(&authRSAPrivateKey.PublicKey)
	if err != nil {
		return nil, "", err
	}

	return newFuncServerOption(func(o *serverOptions) error {
		if o.authKeys == nil {
			o.authKeys = map[string]authKeyData{}
		}
		o.authKeys[thumbprint] = authKeyData{
			id:         thumbprint,
			method:     jwt.SigningMethodRS256,
			privateKey: authRSAPrivateKey,
			publicKey:  &authRSAPrivateKey.PublicKey,
		}
		return nil
	}), thumbprint, nil
}

// WithAuthED25519PrivateKey returns a ServerOption which sets the ed25519 private key to
// use for signed JWTs.
func WithAuthED25519PrivateKey(authED25519PrivateKey ed25519.PrivateKey) (ServerOption, string) {
	pubKey := authED25519PrivateKey.Public()
	keyID := ED25519PublicKeyThumbprint(pubKey.(ed25519.PublicKey))
	return newFuncServerOption(func(o *serverOptions) error {
		if o.authKeys == nil {
			o.authKeys = map[string]authKeyData{}
		}
		o.authKeys[keyID] = authKeyData{
			id:         keyID,
			method:     jwt.SigningMethodEdDSA,
			privateKey: authED25519PrivateKey,
			publicKey:  pubKey,
		}
		return nil
	}), keyID
}

// WithJWTSignerKeyID returns a ServerOption which sets the private key to use for signed
// JWTs by its key ID.
func WithJWTSignerKeyID(keyID string) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.jwtSignerKeyID = keyID
		return nil
	})
}

// WithAuthAudience returns a ServerOption which sets the JWT audience (aud) to
// use/expect in all processed JWTs. When unset, it will be debug logged that
// the instance names will be used instead. It is recommended this option
// is used.
func WithAuthAudience(authAudience ...string) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		if len(authAudience) == 0 {
			return errors.New("expected at least one auth audience member")
		}
		o.authAudience = authAudience
		return nil
	})
}

// WithAuthIssuer returns a ServerOption which sets the JWT issuer (iss) to
// use in all issued JWTs. When unset, it will be debug logged that
// the first audience member will be used instead. It is recommended this option
// is used.
func WithAuthIssuer(authIssuer string) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		if authIssuer == "" {
			return errors.New("auth issuer must be non-empty")
		}
		o.authIssuer = authIssuer
		return nil
	})
}

// WithDebug returns a ServerOption which informs the server to be in a
// debug mode as much as possible.
func WithDebug() ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.debug = true
		return nil
	})
}

// WithTLSAuthHandler returns a ServerOption which when TLS info is available to a connection, it will
// authenticate the given entities in the event that no other authentication has been established via
// the standard auth handler.
func WithTLSAuthHandler(entities []string) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		entityChecker := MakeEntitiesChecker(entities)
		o.tlsAuthHandler = func(ctx context.Context, recvEntities ...string) error {
			if err := entityChecker(ctx, recvEntities...); err != nil {
				return errNotTLSAuthed
			}
			return nil
		}
		return nil
	})
}

// WithAuthHandler returns a ServerOption which adds an auth handler associated
// to the given credential type to use for authentication requests.
func WithAuthHandler(forType CredentialsType, handler AuthHandler) ServerOption {
	return withCredAuthHandlers(forType, credAuthHandlers{
		AuthHandler: handler,
	})
}

// WithEnsureAuthedHandler returns a ServerOptions which adds custom logic for
// the ensuring of authentication on each incoming request. Can only be used
// in testing environments (will produce an error when ensuring authentication
// otherwise).
func WithEnsureAuthedHandler(eah func(ctx context.Context) (context.Context, error)) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.ensureAuthedHandler = eah
		return nil
	})
}

// WithEntityDataLoader returns a ServerOption which adds an entity data loader
// associated to the given credential type to use for loading data after the signed
// access token has been verified.
func WithEntityDataLoader(forType CredentialsType, loader EntityDataLoader) ServerOption {
	return withCredAuthHandlers(forType, credAuthHandlers{
		EntityDataLoader: loader,
	})
}

// WithTokenVerificationKeyProvider returns a ServerOption which adds a token
// verification key provider  associated to the given credential type to use for
// determining an encryption key to verify signed access token prior
// to following through with any RPC methods or entity data loading.
func WithTokenVerificationKeyProvider(forType CredentialsType, provider TokenVerificationKeyProvider) ServerOption {
	return withCredAuthHandlers(forType, credAuthHandlers{
		TokenVerificationKeyProvider: provider,
	})
}

func withCredAuthHandlers(forType CredentialsType, handler credAuthHandlers) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		if forType == credentialsTypeInternal {
			return errors.Errorf("cannot use %q externally", forType)
		}
		if forType == "" {
			return errors.New("type cannot be empty")
		}
		var existingHandler credAuthHandlers
		if o.authHandlersForCreds == nil {
			o.authHandlersForCreds = make(map[CredentialsType]credAuthHandlers)
		} else {
			existingHandler = o.authHandlersForCreds[forType]
		}
		if handler.AuthHandler != nil {
			if existingHandler.AuthHandler != nil {
				return errors.Errorf("%q already has an AuthHandler", forType)
			}
			existingHandler.AuthHandler = handler.AuthHandler
		}
		if handler.EntityDataLoader != nil {
			if existingHandler.EntityDataLoader != nil {
				return errors.Errorf("%q already has an EntityDataLoader", forType)
			}
			existingHandler.EntityDataLoader = handler.EntityDataLoader
		}
		if handler.TokenVerificationKeyProvider != nil {
			if existingHandler.TokenVerificationKeyProvider != nil {
				return errors.Errorf("%q already has an TokenVerificationKeyProvider", forType)
			}
			existingHandler.TokenVerificationKeyProvider = handler.TokenVerificationKeyProvider
		}
		o.authHandlersForCreds[forType] = existingHandler

		return nil
	})
}

// WithExternalAuthRSAPublicKeyTokenVerifier returns a ServerOption to verify all externally
// authenticated entity access tokens with the given public key.
func WithExternalAuthRSAPublicKeyTokenVerifier(pubKey *rsa.PublicKey) ServerOption {
	return WithTokenVerificationKeyProvider(CredentialsTypeExternal, MakeRSAPublicKeyProvider(pubKey))
}

// WithExternalAuthEd25519PublicKeyTokenVerifier returns a ServerOption to verify all externally
// authenticated entity access tokens with the given public key.
func WithExternalAuthEd25519PublicKeyTokenVerifier(pubKey ed25519.PublicKey) ServerOption {
	return WithTokenVerificationKeyProvider(CredentialsTypeExternal, MakeEd25519PublicKeyProvider(pubKey))
}

// WithExternalAuthJWKSetTokenVerifier returns a ServerOption to verify all externally
// authenticated entity access tokens against the given JWK key set.
func WithExternalAuthJWKSetTokenVerifier(keySet jwks.KeySet) ServerOption {
	return WithTokenVerificationKeyProvider(
		CredentialsTypeExternal,
		MakeJWKSKeyProvider(jwks.NewStaticJWKKeyProvider(keySet)))
}

// WithExternalTokenVerificationKeyProvider returns a ServerOption to verify all externally
// authenticated entity access tokens against the given TokenVerificationKeyProvider.
func WithExternalTokenVerificationKeyProvider(provider TokenVerificationKeyProvider) ServerOption {
	return WithTokenVerificationKeyProvider(CredentialsTypeExternal, provider)
}

// WithExternalAuthOIDCTokenVerifier returns a ServerOption to verify all externally
// authenticated entity access tokens against the given OIDC JWT issuer
// that follows the OIDC Discovery protocol.
func WithExternalAuthOIDCTokenVerifier(ctx context.Context, issuer string) (ServerOption, func(ctx context.Context) error, error) {
	provider, err := MakeOIDCKeyProvider(ctx, issuer)
	if err != nil {
		return nil, nil, err
	}
	return WithExternalTokenVerificationKeyProvider(provider), provider.Close, nil
}

// WithAuthenticateToHandler returns a ServerOption which adds an authentication
// handler designed to allow the caller to authenticate itself to some other entity.
// This is useful when externally authenticating as one entity for the purpose of
// getting access to another entity. Only one handler can exist and will always
// produce a credential type of CredentialsTypeExternal.
// This can technically be used internal to the same server to "assume" the identity of
// another entity but is not intended for such usage.
func WithAuthenticateToHandler(handler AuthenticateToHandler) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.authToHandler = handler
		return nil
	})
}

// WithDisableMulticastDNS returns a ServerOption which disables
// using mDNS to broadcast how to connect to this host.
func WithDisableMulticastDNS() ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.disableMDNS = true
		return nil
	})
}

// WithUnknownServiceHandler returns a ServerOption that allows for adding a custom
// unknown service handler. The provided method is a bidi-streaming RPC service
// handler that will be invoked instead of returning the "unimplemented" gRPC
// error whenever a request is received for an unregistered service or method.
// The handling function and stream interceptor (if set) have full access to
// the ServerStream, including its Context.
// See grpc#WithUnknownServiceHandler.
func WithUnknownServiceHandler(streamHandler grpc.StreamHandler) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.unknownStreamDesc = &grpc.StreamDesc{
			StreamName:    "unknown_service_handler",
			Handler:       streamHandler,
			ClientStreams: true,
			ServerStreams: true,
		}
		return nil
	})
}

// WithStatsHandler returns a ServerOption which sets the stats handler on the
// DialOption that specifies the stats handler for all the RPCs and underlying network
// connections.
func WithStatsHandler(handler stats.Handler) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.statsHandler = handler
		return nil
	})
}

// WithAllowUnauthenticatedHealthCheck returns a server option that
// allows the health check to be unauthenticated.
func WithAllowUnauthenticatedHealthCheck() ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.allowUnauthenticatedHealthCheck = true
		return nil
	})
}

// WithPublicMethods returns a server option with grpc methods that can bypass auth validation.
func WithPublicMethods(fullMethods []string) ServerOption {
	return newFuncServerOption(func(o *serverOptions) error {
		o.publicMethods = fullMethods
		return nil
	})
}
