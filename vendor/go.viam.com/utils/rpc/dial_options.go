package rpc

import (
	"crypto/tls"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
)

// dialOptions configure a Dial call. dialOptions are set by the DialOption
// values passed to Dial.
type dialOptions struct {
	// insecure determines if the RPC connection is TLS based.
	insecure bool

	// tlsConfig is the TLS config to use for any secured connections.
	tlsConfig *tls.Config

	// allowInsecureDowngrade determines if it is acceptable to downgrade
	// an insecure connection if detected. This is only used when credentials
	// are not present.
	allowInsecureDowngrade bool

	// allowInsecureWithCredsDowngrade determines if it is acceptable to downgrade
	// an insecure connection if detected when using credentials. This is generally
	// unsafe to use but can be requested.
	allowInsecureWithCredsDowngrade bool

	authEntity string

	// creds are used to authenticate the request. These are orthogonal to insecure,
	// however it's strongly recommended to be on a secure connection when transmitting
	// credentials.
	creds Credentials

	// webrtcOpts control how WebRTC is utilized in a dial attempt.
	webrtcOpts    DialWebRTCOptions
	webrtcOptsSet bool

	externalAuthAddr     string
	externalAuthToEntity string
	externalAuthInsecure bool
	// static auth material used when an external auth service is used. This is also used for the signaler
	// when the webrtc options are empty. See fixupWebRTCOptions.
	externalAuthMaterial string

	// static auth material used when directly connecting to the endpoint. If set all externalAuth options are ignored.
	authMaterial string

	// debug is helpful to turn on when the library isn't working quite right.
	// It will output much more logs.
	debug bool

	mdnsOptions DialMulticastDNSOptions
	// set when the connection is using mdns flow.
	usingMDNS bool

	disableDirect bool

	// stats monitoring on the connections.
	statsHandler stats.Handler

	// interceptors
	unaryInterceptor  grpc.UnaryClientInterceptor
	streamInterceptor grpc.StreamClientInterceptor

	// signalingConn can be used to force the webrtcSignalingAnswerer to use a preexisting connection instead of dialing and managing its own.
	signalingConn ClientConn
}

// DialMulticastDNSOptions dictate any special settings to apply while dialing via mDNS.
type DialMulticastDNSOptions struct {
	// Disable disables mDNS service discovery for other robots. You may want to use this
	// if you do not trust the network you're in to truthfully advertise services. That
	// being said, if this is a concern, you should use TLS server verification.
	Disable bool

	// RemoveAuthCredentials will remove any and all authentication credentials when dialing.
	// This is particularly helpful in managed environments that do inter-robot TLS authN/Z.
	RemoveAuthCredentials bool
}

// DialOption configures how we set up the connection.
// Cribbed from https://github.com/grpc/grpc-go/blob/aff571cc86e6e7e740130dbbb32a9741558db805/dialoptions.go#L41
type DialOption interface {
	apply(*dialOptions)
}

// funcDialOption wraps a function that modifies dialOptions into an
// implementation of the DialOption interface.
type funcDialOption struct {
	f func(*dialOptions)
}

func (fdo *funcDialOption) apply(do *dialOptions) {
	fdo.f(do)
}

func newFuncDialOption(f func(*dialOptions)) *funcDialOption {
	return &funcDialOption{
		f: f,
	}
}

// WithInsecure returns a DialOption which disables transport security for this
// ClientConn. Note that transport security is required unless WithInsecure is
// set.
func WithInsecure() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.insecure = true
	})
}

// WithCredentials returns a DialOption which sets the credentials to use for
// authenticating the request. The associated entity is assumed to be the
// address of the server. This is mutually exclusive with
// WithEntityCredentials.
func WithCredentials(creds Credentials) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.authEntity = ""
		o.creds = creds
	})
}

// WithEntityCredentials returns a DialOption which sets the entity credentials
// to use for authenticating the request. This is mutually exclusive with
// WithCredentials.
func WithEntityCredentials(entity string, creds Credentials) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.authEntity = entity
		o.creds = creds
	})
}

// WithExternalAuth returns a DialOption which sets the address to use
// to perform authentication. Authentication done in this manner will never
// have the dialed address be authenticated against but instead have access
// tokens sent directly to it. The entity which authentication is intended for
// must also be specified. ExternalAuth uses the ExternalAuthService extension
// and this library does not provide a standard implementation for it. It is
// expected that the credentials used in these same dial options will be used
// to first authenticate to the external server using the AuthService.
// Note: When making a gRPC connection to the given address, the same
// dial options are used. That means if the external address is secured,
// so must the internal target.
func WithExternalAuth(addr, toEntity string) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.externalAuthAddr = addr
		o.externalAuthToEntity = toEntity
	})
}

// WithExternalAuthInsecure returns a DialOption which disables transport security for this
// ClientConn when doing external auth. Note that transport security is required unless
// WithExternalAuthInsecure is set.
func WithExternalAuthInsecure() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.externalAuthInsecure = true
	})
}

// WithStaticAuthenticationMaterial returns a DialOption which sets the already authenticated
// material (opaque) to use for authenticated requests. This is mutually exclusive with
// auth and external auth options.
func WithStaticAuthenticationMaterial(authMaterial string) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.authEntity = ""
		o.creds = Credentials{}
		o.authMaterial = authMaterial
	})
}

// WithStaticExternalAuthenticationMaterial returns a DialOption which sets the already authenticated
// material (opaque) to use for authenticated requests against an external auth service. Ignored if
// WithStaticAuthenticationMaterial() is set.
func WithStaticExternalAuthenticationMaterial(authMaterial string) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.authEntity = ""
		o.creds = Credentials{}
		o.externalAuthMaterial = authMaterial
	})
}

// WithTLSConfig sets the TLS configuration to use for all secured connections.
func WithTLSConfig(config *tls.Config) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.tlsConfig = config
	})
}

// WithWebRTCOptions returns a DialOption which sets the WebRTC options
// to use if the dialer tries to establish a WebRTC connection.
func WithWebRTCOptions(webrtcOpts DialWebRTCOptions) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.webrtcOpts = webrtcOpts
		o.webrtcOptsSet = true
	})
}

// WithDialDebug returns a DialOption which informs the client to be in a
// debug mode as much as possible.
func WithDialDebug() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.debug = true
	})
}

// WithAllowInsecureDowngrade returns a DialOption which allows connections
// to be downgraded to plaintext if TLS cannot be detected properly. This
// is not used when there are credentials present. For that, use the
// more explicit WithAllowInsecureWithCredsDowngrade.
func WithAllowInsecureDowngrade() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.allowInsecureDowngrade = true
	})
}

// WithAllowInsecureWithCredentialsDowngrade returns a DialOption which allows
// connections to be downgraded to plaintext if TLS cannot be detected properly while
// using credentials. This is generally unsafe to use but can be requested.
func WithAllowInsecureWithCredentialsDowngrade() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.allowInsecureWithCredsDowngrade = true
	})
}

// WithDialMulticastDNSOptions returns a DialOption which allows setting
// options to specifically be used while doing a dial based off mDNS
// discovery.
func WithDialMulticastDNSOptions(opts DialMulticastDNSOptions) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.mdnsOptions = opts
	})
}

// WithDisableDirectGRPC returns a DialOption which disables directly dialing a gRPC server.
// There's not really a good reason to use this unless it's for testing.
func WithDisableDirectGRPC() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.disableDirect = true
	})
}

// WithDialStatsHandler returns a DialOption which sets the stats handler on the
// DialOption that specifies the stats handler for all the RPCs and underlying network
// connections.
func WithDialStatsHandler(handler stats.Handler) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.statsHandler = handler
	})
}

// WithUnaryClientInterceptor returns a DialOption that specifies the interceptor for
// unary RPCs.
func WithUnaryClientInterceptor(interceptor grpc.UnaryClientInterceptor) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		if o.unaryInterceptor != nil {
			o.unaryInterceptor = grpc_middleware.ChainUnaryClient(o.unaryInterceptor, interceptor)
		} else {
			o.unaryInterceptor = interceptor
		}
	})
}

// WithStreamClientInterceptor returns a DialOption that specifies the interceptor for
// streaming RPCs.
func WithStreamClientInterceptor(interceptor grpc.StreamClientInterceptor) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		if o.streamInterceptor != nil {
			o.streamInterceptor = grpc_middleware.ChainStreamClient(
				o.streamInterceptor,
				interceptor,
			)
		} else {
			o.streamInterceptor = interceptor
		}
	})
}

// WithForceDirectGRPC forces direct dialing to the target address. This option disables WebRTC connections and mDNS lookup.
func WithForceDirectGRPC() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.mdnsOptions.Disable = true
		o.webrtcOpts.Disable = true
		o.disableDirect = false
	})
}

// WithSignalingConn provides a preexisting connection to use. This option forces the webrtcSignalingAnswerer to not dial or manage
// a connection.
func WithSignalingConn(signalingConn ClientConn) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.signalingConn = signalingConn
	})
}
