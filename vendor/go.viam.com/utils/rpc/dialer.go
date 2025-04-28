package rpc

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/pkg/errors"
	"github.com/viamrobotics/webrtc/v3"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/utils"
	rpcpb "go.viam.com/utils/proto/rpc/v1"
)

// create a new TLS config with the default options for RPC.
func newDefaultTLSConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12}
}

// A Dialer is responsible for making connections to gRPC endpoints.
type Dialer interface {
	// DialDirect makes a connection to the given target over standard gRPC with the supplied options.
	DialDirect(
		ctx context.Context,
		target string,
		keyExtra string,
		onClose func() error,
		opts ...grpc.DialOption,
	) (conn ClientConn, cached bool, err error)

	// DialFunc makes a connection to the given target for the given proto using the given dial function.
	DialFunc(
		proto string,
		target string,
		keyExtra string,
		dialNew func() (ClientConn, func() error, error),
	) (conn ClientConn, cached bool, err error)

	// Close ensures all connections made are cleanly closed.
	Close() error
}

// A ClientConn is a wrapper around the gRPC client connection interface but ensures
// there is a way to close the connection.
type ClientConn interface {
	grpc.ClientConnInterface

	// PeerConn returns the backing PeerConnection object, or nil if the underlying transport is not
	// a PeerConnection.
	PeerConn() *webrtc.PeerConnection
	Close() error
}

// A ClientConnAuthenticator supports instructing a connection to authenticate now.
type ClientConnAuthenticator interface {
	ClientConn
	Authenticate(ctx context.Context) (string, error)
}

// A GrpcOverHTTPClientConn is grpc connection that is not backed by a `webrtc.PeerConnection`.
type GrpcOverHTTPClientConn struct {
	*grpc.ClientConn
}

// PeerConn returns nil as this is a native gRPC connection.
func (cc GrpcOverHTTPClientConn) PeerConn() *webrtc.PeerConnection {
	return nil
}

type cachedDialer struct {
	mu    sync.Mutex // Note(erd): not suitable for highly concurrent usage
	conns map[string]*refCountedConnWrapper
}

// NewCachedDialer returns a Dialer that returns the same connection if it
// already has been established at a particular target (regardless of the
// options used).
func NewCachedDialer() Dialer {
	return &cachedDialer{conns: map[string]*refCountedConnWrapper{}}
}

func (cd *cachedDialer) DialDirect(
	ctx context.Context,
	target string,
	keyExtra string,
	onClose func() error,
	opts ...grpc.DialOption,
) (ClientConn, bool, error) {
	return cd.DialFunc("grpc", target, keyExtra, func() (ClientConn, func() error, error) {
		conn, err := grpc.DialContext(ctx, target, opts...) //nolint:staticcheck
		if err != nil {
			return nil, nil, err
		}
		return GrpcOverHTTPClientConn{conn}, onClose, nil
	})
}

func (cd *cachedDialer) DialFunc(
	proto string,
	target string,
	keyExtra string,
	dialNew func() (ClientConn, func() error, error),
) (ClientConn, bool, error) {
	key := fmt.Sprintf("%s:%s:%s", proto, target, keyExtra)
	cd.mu.Lock()
	c, ok := cd.conns[key]
	cd.mu.Unlock()
	if ok {
		return c.Ref(), true, nil
	}

	// assume any difference in opts does not matter
	conn, onClose, err := dialNew()
	if err != nil {
		return nil, false, err
	}
	conn = wrapClientConnWithCloseFunc(conn, onClose)
	refConn := newRefCountedConnWrapper(proto, conn, func() {
		cd.mu.Lock()
		delete(cd.conns, key)
		cd.mu.Unlock()
	})
	cd.mu.Lock()
	defer cd.mu.Unlock()

	// someone else might have already connected
	c, ok = cd.conns[key]
	if ok {
		if err := conn.Close(); err != nil {
			return nil, false, err
		}
		return c.Ref(), true, nil
	}
	cd.conns[key] = refConn
	return refConn.Ref(), false, nil
}

func (cd *cachedDialer) Close() error {
	cd.mu.Lock()
	// need a copy of cd.conns as we can't hold the lock, since .Close() fires the onUnref() set (above) in DialFunc()
	// that uses the same lock and directly modifies cd.conns when the dialer is reused at different layers (e.g. auth and multi)
	var conns []*refCountedConnWrapper
	for _, c := range cd.conns {
		conns = append(conns, c)
	}
	cd.mu.Unlock()
	var err error
	for _, c := range conns {
		if closeErr := c.actual.Close(); closeErr != nil && status.Convert(closeErr).Code() != codes.Canceled {
			err = multierr.Combine(err, closeErr)
		}
	}
	return err
}

// newRefCountedConnWrapper wraps the given connection to be able to be reference counted.
func newRefCountedConnWrapper(proto string, conn ClientConn, onUnref func()) *refCountedConnWrapper {
	return &refCountedConnWrapper{proto, utils.NewRefCountedValue(conn), conn, onUnref}
}

// refCountedConnWrapper wraps a ClientConn to be reference counted.
type refCountedConnWrapper struct {
	proto   string
	ref     utils.RefCountedValue
	actual  ClientConn
	onUnref func()
}

// Ref returns a new reference to the underlying ClientConn.
func (w *refCountedConnWrapper) Ref() ClientConn {
	return &reffedConn{ClientConn: w.ref.Ref().(ClientConn), proto: w.proto, deref: w.ref.Deref, onUnref: w.onUnref}
}

// A reffedConn reference counts a ClieentConn and closes it on the last dereference.
type reffedConn struct {
	ClientConn
	proto     string
	derefOnce sync.Once
	deref     func() bool
	onUnref   func()
}

// Close will deref the reference and if it is the last to do so, will close
// the underlying connection.
func (rc *reffedConn) Close() error {
	var err error
	rc.derefOnce.Do(func() {
		if unref := rc.deref(); unref {
			if rc.onUnref != nil {
				defer rc.onUnref()
			}
			if utils.Debug {
				golog.Global().Debugw("close referenced conn", "proto", rc.proto)
			}
			if pc := rc.ClientConn.PeerConn(); pc != nil {
				utils.UncheckedErrorFunc(pc.GracefulClose)
			}
			if closeErr := rc.ClientConn.Close(); closeErr != nil && status.Convert(closeErr).Code() != codes.Canceled {
				err = closeErr
			}
		}
	})
	return err
}

// ErrInsecureWithCredentials is sent when a dial attempt is made to an address where either the insecure
// option or insecure downgrade with credentials options are not set.
var ErrInsecureWithCredentials = errors.New("requested address is insecure and will not send credentials")

// DialDirectGRPC dials a gRPC server directly.
func DialDirectGRPC(ctx context.Context, address string, logger utils.ZapCompatibleLogger, opts ...DialOption) (ClientConn, error) {
	var dOpts dialOptions
	for _, opt := range opts {
		opt.apply(&dOpts)
	}
	dOpts.webrtcOpts.Disable = true
	dOpts.mdnsOptions.Disable = true

	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	return dialInner(ctx, address, logger, dOpts)
}

func socksProxyDialContext(ctx context.Context, network, proxyAddr, addr string) (net.Conn, error) {
	dialer, err := proxy.SOCKS5(network, proxyAddr, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("error creating SOCKS proxy dialer to address %q from environment: %w",
			proxyAddr, err)
	}
	return dialer.(proxy.ContextDialer).DialContext(ctx, network, addr)
}

// SocksProxyFallbackDialContext will return nil if SocksProxyEnvVar is not set or if trying to connect to a local address,
// which will allow dialers to use the default DialContext.
// If SocksProxyEnvVar is set, it will prioritize a connection made without a proxy but will fall back to a SOCKS proxy connection.
func SocksProxyFallbackDialContext(
	addr string, logger utils.ZapCompatibleLogger,
) func(ctx context.Context, network, addr string) (net.Conn, error) {
	// Use SOCKS proxy from environment as gRPC proxy dialer. Do not use SOCKS proxy if trying to connect to a local address.
	localAddr := strings.HasPrefix(addr, "[::]") || strings.HasPrefix(addr, "localhost") || strings.HasPrefix(addr, "unix")
	proxyAddr := os.Getenv(SocksProxyEnvVar)
	if localAddr || proxyAddr == "" {
		// return nil in these cases so that the default dialer gets used instead.
		return nil
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// if ONLY_SOCKS_PROXY specified, no need for a parallel dial - only dial through
		// the SOCKS proxy directly.
		if os.Getenv(OnlySocksProxyEnvVar) != "" {
			logger.Infow("Both SOCKS_PROXY and ONLY_SOCKS_PROXY specified, only SOCKS proxy will be used for outgoing connection")
			conn, err := socksProxyDialContext(ctx, network, proxyAddr, addr)
			if err == nil {
				logger.Infow("connected with SOCKS proxy")
			}
			return conn, err
		}

		// the block below heavily references https://go.dev/src/net/dial.go#L585
		type dialResult struct {
			net.Conn
			error
			primary bool
			done    bool
		}
		results := make(chan dialResult) // unbuffered

		var primary, fallback dialResult
		var wg sync.WaitGroup
		defer wg.Wait()

		// otherwise, do a parallel dial with a slight delay for the fallback option.
		returned := make(chan struct{})
		defer close(returned)

		dialer := func(ctx context.Context, dialFunc func(context.Context) (net.Conn, error), primary bool) {
			defer wg.Done()
			conn, err := dialFunc(ctx)
			select {
			case results <- dialResult{Conn: conn, error: err, primary: primary, done: true}:
			case <-returned:
				if conn != nil {
					utils.UncheckedError(conn.Close())
				}
			}
		}

		logger.Infow("SOCKS_PROXY specified, SOCKS proxy will be used as a fallback for outgoing connection")
		// start the main dial attempt.
		primaryCtx, primaryCancel := context.WithCancel(ctx)
		defer primaryCancel()
		wg.Add(1)
		primaryDial := func(ctx context.Context) (net.Conn, error) {
			// create a zero-valued net.Dialer to use net.Dialer's default DialContext method
			var zeroDialer net.Dialer
			return zeroDialer.DialContext(ctx, network, addr)
		}
		go dialer(primaryCtx, primaryDial, true)

		// wait a small amount before starting the fallback dial (to prioritize the primary connection method).
		fallbackTimer := time.NewTimer(300 * time.Millisecond)
		defer fallbackTimer.Stop()

		// fallbackCtx is defined here because this fails `go vet` otherwise. The intent is for fallbackCancel
		// to be called as this function exits, which will cancel the ongoing SOCKS proxy if it is still running.
		fallbackCtx, fallbackCancel := context.WithCancel(ctx)
		defer fallbackCancel()

		// a for loop is used here so that we wait on both results and the fallback timer at the same time.
		// if the timer expires, we should start the fallback dial and then wait for results.
		// if the results channel receives a message, the message should be processed and either return
		// or continue waiting (and reset the timer if it hasn't already expired).
		for {
			select {
			case <-fallbackTimer.C:
				wg.Add(1)
				fallbackDial := func(ctx context.Context) (net.Conn, error) {
					return socksProxyDialContext(ctx, network, proxyAddr, addr)
				}
				go dialer(fallbackCtx, fallbackDial, false)
			case res := <-results:
				if res.error == nil {
					if res.primary {
						logger.Infow("connected with ethernet/wifi")
					} else {
						logger.Infow("connected with SOCKS proxy")
					}
					return res.Conn, nil
				}
				if res.primary {
					primary = res
				} else {
					fallback = res
				}
				// if both primary and fallback are done with errors, this means neither connection attempt succeeded.
				// return the error from the primary dial attempt in that case.
				if primary.done && fallback.done {
					return nil, primary.error
				}
				if res.primary && fallbackTimer.Stop() {
					// If we were able to stop the timer, that means it
					// was running (hadn't yet started the fallback), but
					// we just got an error on the primary path, so start
					// the fallback immediately (in 0 nanoseconds).
					fallbackTimer.Reset(0)
				}
			}
		}
	}
}

// dialDirectGRPC dials a gRPC server directly.
func dialDirectGRPC(ctx context.Context, address string, dOpts dialOptions, logger utils.ZapCompatibleLogger) (ClientConn, bool, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithBlock(), //nolint:staticcheck
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                keepAliveTime * 2, // a little extra buffer to try to avoid ENHANCE_YOUR_CALM
			PermitWithoutStream: true,
		}),
	}

	// check if we should use a custom dialer that will use the SOCKS proxy as a fallback. Only attach a new context dialer
	// if the returned function is not nil.
	//
	// use "tcp" since gRPC uses HTTP/2, which is built on top of TCP.
	socksProxyDialContext := SocksProxyFallbackDialContext(address, logger)
	if socksProxyDialContext != nil {
		dialOpts = append(dialOpts, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return socksProxyDialContext(ctx, "tcp", addr)
		}))
	}

	if dOpts.insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig := dOpts.tlsConfig
		if tlsConfig == nil {
			tlsConfig = newDefaultTLSConfig()
		}

		var downgrade bool
		if dOpts.allowInsecureDowngrade || dOpts.allowInsecureWithCredsDowngrade {
			var dialer tls.Dialer
			dialer.Config = tlsConfig
			conn, err := dialer.DialContext(ctx, "tcp", address)
			if err == nil {
				// will use TLS
				utils.UncheckedError(conn.Close())
			} else if strings.Contains(err.Error(), "tls: first record does not look like a TLS handshake") {
				// unfortunately there's no explicit error value for this, so we do a string check
				hasLocalCreds := dOpts.creds.Type != "" && dOpts.externalAuthAddr == ""
				if dOpts.creds.Type == "" || !hasLocalCreds || dOpts.allowInsecureWithCredsDowngrade {
					logger.Warnw("downgrading from TLS to plaintext", "address", address, "with_credentials", hasLocalCreds)
					downgrade = true
				} else if hasLocalCreds {
					return nil, false, ErrInsecureWithCredentials
				}
			}
		}
		if downgrade {
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		}
	}

	if dOpts.statsHandler != nil {
		dialOpts = append(dialOpts, grpc.WithStatsHandler(dOpts.statsHandler))
	}

	grpcLogger := logger.Desugar()
	if !(dOpts.debug || utils.Debug) {
		grpcLogger = grpcLogger.WithOptions(zap.IncreaseLevel(zap.LevelEnablerFunc(zapcore.ErrorLevel.Enabled)))
	}
	var unaryInterceptors []grpc.UnaryClientInterceptor
	unaryInterceptors = append(unaryInterceptors, grpc_zap.UnaryClientInterceptor(grpcLogger))
	unaryInterceptors = append(unaryInterceptors, UnaryClientTracingInterceptor())
	if dOpts.unaryInterceptor != nil {
		unaryInterceptors = append(unaryInterceptors, dOpts.unaryInterceptor)
	}

	var streamInterceptors []grpc.StreamClientInterceptor
	streamInterceptors = append(streamInterceptors, grpc_zap.StreamClientInterceptor(grpcLogger))
	streamInterceptors = append(streamInterceptors, StreamClientTracingInterceptor())
	if dOpts.streamInterceptor != nil {
		streamInterceptors = append(streamInterceptors, dOpts.streamInterceptor)
	}

	var connPtr *ClientConn
	var closeCredsFunc func() error
	var rpcCreds *perRPCJWTCredentials

	if dOpts.authMaterial != "" {
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(&staticPerRPCJWTCredentials{dOpts.authMaterial}))
	} else if dOpts.creds.Type != "" || dOpts.externalAuthMaterial != "" {
		rpcCreds = &perRPCJWTCredentials{
			entity: dOpts.authEntity,
			creds:  dOpts.creds,
			debug:  dOpts.debug,
			logger: logger,
			// Note: don't set dialOptsCopy.authMaterial below as perRPCJWTCredentials will know to use
			// its externalAccessToken to authenticateTo. This will result in both a connection level authorization
			// added as well as an authorization header added from perRPCJWTCredentials, resulting in a failure.
			externalAuthMaterial: dOpts.externalAuthMaterial,
		}
		if dOpts.debug && dOpts.externalAuthAddr != "" && dOpts.externalAuthToEntity != "" {
			logger.Debugw("will eventually authenticate as entity", "entity", dOpts.authEntity)
		}
		if dOpts.externalAuthAddr != "" {
			externalConn, err := dialExternalAuthEntity(ctx, logger, dOpts)
			if err != nil {
				return nil, false, err
			}

			closeCredsFunc = externalConn.Close
			rpcCreds.conn = externalConn
			rpcCreds.externalAuthToEntity = dOpts.externalAuthToEntity
		} else {
			connPtr = &rpcCreds.conn
		}
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(rpcCreds))
		unaryInterceptors = append(unaryInterceptors, UnaryClientInvalidAuthInterceptor(rpcCreds))
		// InvalidAuthInterceptor will not work for streaming calls; we can only
		// intercept the creation of a stream, and the ensuring of authentication
		// server-side happens per RPC request (per usage of the stream).
	}

	unaryInterceptor := grpc_middleware.ChainUnaryClient(unaryInterceptors...)
	dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(unaryInterceptor))
	streamInterceptor := grpc_middleware.ChainStreamClient(streamInterceptors...)
	dialOpts = append(dialOpts, grpc.WithStreamInterceptor(streamInterceptor))

	var conn ClientConn
	var cached bool
	var err error
	if ctxDialer := contextDialer(ctx); ctxDialer != nil {
		conn, cached, err = ctxDialer.DialDirect(ctx, address, dOpts.cacheKey(), closeCredsFunc, dialOpts...)
	} else {
		var grpcConn *grpc.ClientConn
		grpcConn, err = grpc.DialContext(ctx, address, dialOpts...) //nolint:staticcheck
		if err == nil {
			conn = GrpcOverHTTPClientConn{grpcConn}
		}
		if err == nil && closeCredsFunc != nil {
			conn = wrapClientConnWithCloseFunc(conn, closeCredsFunc)
		}
	}
	if err != nil {
		if closeCredsFunc != nil {
			err = multierr.Combine(err, closeCredsFunc())
		}
		return nil, false, err
	}
	if connPtr != nil {
		*connPtr = conn
	}
	if rpcCreds != nil {
		conn = clientConnRPCAuthenticator{conn, rpcCreds}
	}
	return conn, cached, err
}

func dialExternalAuthEntity(ctx context.Context, logger utils.ZapCompatibleLogger, dOpts dialOptions) (ClientConn, error) {
	if dOpts.externalAuthToEntity == "" {
		return nil, errors.New("external auth address set but no authenticate to option set")
	}
	if dOpts.debug {
		logger.Debugw("will eventually authenticate externally to entity", "entity", dOpts.externalAuthToEntity)
		logger.Debugw("dialing direct for external auth", "address", dOpts.externalAuthAddr)
	}

	address := dOpts.externalAuthAddr
	dOpts.externalAuthAddr = ""

	dOpts.insecure = dOpts.externalAuthInsecure
	dOpts.externalAuthMaterial = ""
	dOpts.creds = Credentials{}
	dOpts.authEntity = ""

	// reset the tls config that is used for the external Auth Service.
	dOpts.tlsConfig = newDefaultTLSConfig()

	conn, cached, err := dialDirectGRPC(ctx, address, dOpts, logger)
	if dOpts.debug {
		logger.Debugw("connected directly for external auth", "address", address, "cached", cached)
	}
	return conn, err
}

// cacheKey hashes options to only cache when authentication material
// is the same between dials. That means any time a new way that differs
// authentication based on options is introduced, this function should
// also be updated.
func (dOpts dialOptions) cacheKey() string {
	hasher := fnv.New128a()
	if dOpts.authEntity != "" {
		hasher.Write([]byte(dOpts.authEntity))
	}
	if dOpts.creds.Type != "" {
		hasher.Write([]byte(dOpts.creds.Type))
	}
	if dOpts.creds.Payload != "" {
		hasher.Write([]byte(dOpts.creds.Payload))
	}
	if dOpts.externalAuthAddr != "" {
		hasher.Write([]byte(dOpts.externalAuthAddr))
	}
	if dOpts.externalAuthToEntity != "" {
		hasher.Write([]byte(dOpts.externalAuthToEntity))
	}
	if dOpts.externalAuthMaterial != "" {
		hasher.Write([]byte(dOpts.externalAuthMaterial))
	}
	if dOpts.webrtcOpts.SignalingServerAddress != "" {
		hasher.Write([]byte(dOpts.webrtcOpts.SignalingServerAddress))
	}
	if dOpts.webrtcOpts.SignalingExternalAuthAddress != "" {
		hasher.Write([]byte(dOpts.webrtcOpts.SignalingExternalAuthAddress))
	}
	if dOpts.webrtcOpts.SignalingExternalAuthToEntity != "" {
		hasher.Write([]byte(dOpts.webrtcOpts.SignalingExternalAuthToEntity))
	}
	if dOpts.webrtcOpts.SignalingExternalAuthAuthMaterial != "" {
		hasher.Write([]byte(dOpts.webrtcOpts.SignalingExternalAuthAuthMaterial))
	}
	if dOpts.webrtcOpts.SignalingCreds.Type != "" {
		hasher.Write([]byte(dOpts.webrtcOpts.SignalingCreds.Type))
	}
	if dOpts.webrtcOpts.SignalingCreds.Payload != "" {
		hasher.Write([]byte(dOpts.webrtcOpts.SignalingCreds.Payload))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func wrapClientConnWithCloseFunc(conn ClientConn, closeFunc func() error) ClientConn {
	return &clientConnWithCloseFunc{ClientConn: conn, closeFunc: closeFunc}
}

type clientConnWithCloseFunc struct {
	ClientConn
	closeFunc func() error
}

func (cc *clientConnWithCloseFunc) Close() (err error) {
	defer func() {
		if cc.closeFunc == nil {
			return
		}
		err = multierr.Combine(err, cc.closeFunc())
	}()
	if pc := cc.ClientConn.PeerConn(); pc != nil {
		utils.UncheckedErrorFunc(pc.GracefulClose)
	}
	return cc.ClientConn.Close()
}

// dialFunc dials an address for a particular protocol and dial function.
func dialFunc(
	ctx context.Context,
	proto string,
	target string,
	keyExtra string,
	f func() (ClientConn, error),
) (ClientConn, bool, error) {
	if ctxDialer := contextDialer(ctx); ctxDialer != nil {
		return ctxDialer.DialFunc(proto, target, keyExtra, func() (ClientConn, func() error, error) {
			conn, err := f()
			return conn, nil, err
		})
	}
	conn, err := f()
	return conn, false, err
}

type clientConnRPCAuthenticator struct {
	ClientConn
	rpcCreds *perRPCJWTCredentials
}

func (cc clientConnRPCAuthenticator) Authenticate(ctx context.Context) (string, error) {
	return cc.rpcCreds.authenticate(ctx)
}

type staticPerRPCJWTCredentials struct {
	authMaterial string
}

func (creds *staticPerRPCJWTCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	for _, uriVal := range uri {
		if strings.HasSuffix(uriVal, "/proto.rpc.v1.AuthService") {
			//nolint:nilnil
			return nil, nil
		}
	}

	return map[string]string{"Authorization": "Bearer " + creds.authMaterial}, nil
}

func (creds *staticPerRPCJWTCredentials) RequireTransportSecurity() bool {
	return false
}

type perRPCJWTCredentials struct {
	mu                   sync.RWMutex
	conn                 ClientConn
	entity               string
	externalAuthToEntity string
	creds                Credentials
	accessToken          string
	// The static external auth material used against the AuthenticateTo request to obtain final accessToken
	externalAuthMaterial string

	debug  bool
	logger utils.ZapCompatibleLogger
}

// TODO(GOUT-10): handle expiration.
func (creds *perRPCJWTCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	for _, uriVal := range uri {
		if strings.HasSuffix(uriVal, "/proto.rpc.v1.AuthService") {
			//nolint:nilnil
			return nil, nil
		}
	}
	accessToken, err := creds.authenticate(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]string{"Authorization": "Bearer " + accessToken}, nil
}

func (creds *perRPCJWTCredentials) authenticate(ctx context.Context) (string, error) {
	creds.mu.RLock()
	accessToken := creds.accessToken
	creds.mu.RUnlock()
	if accessToken == "" {
		creds.mu.Lock()
		defer creds.mu.Unlock()
		accessToken = creds.accessToken
		if accessToken == "" {
			// skip authenticate call when a static access token for the external auth is used.
			if creds.externalAuthMaterial == "" {
				if creds.debug {
					creds.logger.Debugw("authenticating as entity", "entity", creds.entity)
				}
				authClient := rpcpb.NewAuthServiceClient(creds.conn)

				// Check external auth creds...
				resp, err := authClient.Authenticate(ctx, &rpcpb.AuthenticateRequest{
					Entity: creds.entity,
					Credentials: &rpcpb.Credentials{
						Type:    string(creds.creds.Type),
						Payload: creds.creds.Payload,
					},
				})
				if err != nil {
					return "", err
				}
				accessToken = resp.GetAccessToken()
			} else {
				accessToken = creds.externalAuthMaterial
			}

			// now perform external auth
			if creds.externalAuthToEntity == "" {
				if creds.debug {
					creds.logger.Debug("not external auth for an entity; done")
				}
				creds.accessToken = accessToken
			} else {
				if creds.debug {
					creds.logger.Debugw("authenticating to external entity", "entity", creds.externalAuthToEntity)
				}
				// now perform external auth
				md := make(metadata.MD)
				bearer := fmt.Sprintf("Bearer %s", accessToken)
				md.Set("authorization", bearer)
				externalCtx := metadata.NewOutgoingContext(ctx, md)

				externalAuthClient := rpcpb.NewExternalAuthServiceClient(creds.conn)
				externalResp, err := externalAuthClient.AuthenticateTo(externalCtx, &rpcpb.AuthenticateToRequest{
					Entity: creds.externalAuthToEntity,
				})
				if err != nil {
					return "", err
				}

				if creds.debug {
					creds.logger.Debugw("external auth done", "auth_to", creds.externalAuthToEntity)
				}

				accessToken = externalResp.GetAccessToken()
				creds.accessToken = externalResp.GetAccessToken()
			}
		}
	}

	return accessToken, nil
}

func (creds *perRPCJWTCredentials) RequireTransportSecurity() bool {
	return false
}
