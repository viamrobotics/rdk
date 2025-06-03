package rpc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/pkg/errors"
	"github.com/viamrobotics/zeroconf"
	"go.uber.org/multierr"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/utils"
	rpcpb "go.viam.com/utils/proto/rpc/v1"
	webrtcpb "go.viam.com/utils/proto/rpc/webrtc/v1"
)

const (
	mDNSerr           = "mDNS setup failed; continuing with mDNS disabled"
	healthCheckMethod = "/grpc.health.v1.Health/Check"
	healthWatchMethod = "/grpc.health.v1.Health/Watch"
)

// A Server provides a convenient way to get a gRPC server up and running
// with HTTP facilities.
type Server interface {
	// InternalAddr returns the address from the listener used for
	// gRPC communications. It may be the same listener the server
	// was constructed with.
	InternalAddr() net.Addr

	// InstanceNames are the instance names this server claims to be. Typically
	// set via options.
	InstanceNames() []string

	// Start only starts up the internal gRPC server.
	Start() error

	// Serve will externally serve, on the given listener, the
	// all in one handler described by http.Handler.
	Serve(listener net.Listener) error

	// ServeTLS will externally serve, using the given cert/key, the
	// all in one handler described by http.Handler. The provided tlsConfig
	// will be used for any extra TLS settings. If using mutual TLS authentication
	// (see WithTLSAuthHandler), then the tls.Config should have ClientAuth,
	// at a minimum, set to tls.VerifyClientCertIfGiven.
	ServeTLS(listener net.Listener, certFile, keyFile string, tlsConfig *tls.Config) error

	// Stop stops the internal gRPC and the HTTP server if it
	// was started.
	Stop() error

	// RegisterServiceServer associates a service description with
	// its implementation along with any gateway handlers.
	RegisterServiceServer(
		ctx context.Context,
		svcDesc *grpc.ServiceDesc,
		svcServer interface{},
		svcHandlers ...RegisterServiceHandlerFromEndpointFunc,
	) error

	// GatewayHandler returns a handler for gateway based gRPC requests.
	// See: https://github.com/grpc-ecosystem/grpc-gateway
	GatewayHandler() http.Handler

	// GRPCHandler returns a handler for standard grpc/grpc-web requests which
	// expect to be served from a root path.
	GRPCHandler() http.Handler

	// http.Handler implemented here is an all-in-one handler for any kind of gRPC traffic.
	// This is useful in a scenario where all gRPC is served from the root path due to
	// limitations of normal gRPC being served from a non-root path.
	http.Handler

	EnsureAuthed(ctx context.Context) (context.Context, error)

	// Stats returns a structure containing numbers that can be interesting for graphing over time
	// as a diagnostics tool.
	Stats() any
}

type simpleServer struct {
	rpcpb.UnimplementedAuthServiceServer
	rpcpb.UnimplementedExternalAuthServiceServer
	mu                      sync.RWMutex
	activeBackgroundWorkers sync.WaitGroup
	grpcListener            net.Listener
	grpcServer              *grpc.Server
	grpcWebServer           *grpcweb.WrappedGrpcServer
	grpcGatewayHandler      *runtime.ServeMux
	httpServer              *http.Server
	instanceNames           []string
	webrtcServer            *webrtcServer
	webrtcAnswerers         []*webrtcSignalingAnswerer
	serviceServerCancels    []func()
	signalingCallQueue      WebRTCCallQueue
	signalingServer         *WebRTCSignalingServer
	mdnsServers             []*zeroconf.Server
	// exempt methods do not perform any auth
	exemptMethods map[string]bool
	// public methods attempt, but do not require, authentication
	publicMethods        map[string]bool
	tlsConfig            *tls.Config
	firstSeenTLSCertLeaf *x509.Certificate
	stopped              bool
	logger               utils.ZapCompatibleLogger

	// auth
	authKeys             map[string]authKeyData
	authKeyForJWTSigning authKeyData
	internalUUID         string
	internalCreds        Credentials
	tlsAuthHandler       func(ctx context.Context, entities ...string) error
	authHandlersForCreds map[CredentialsType]credAuthHandlers
	authToHandler        AuthenticateToHandler
	ensureAuthedHandler  func(ctx context.Context) (context.Context, error)

	// authAudience is the JWT audience (aud) that will be used/expected
	// for our service.
	authAudience []string

	// authIssuer is the JWT issuer (iss) that will be used for our service.
	authIssuer string

	// counters are for reporting FTDC metrics. A `simpleServer` sets up both a grpc server wrapping
	// a standard http2 over TCP connection. And it also sets up grpc services for webrtc
	// PeerConnections. These counters are specifically for requests coming in over TCP.
	counters struct {
		TCPGrpcRequestsStarted      atomic.Int64
		TCPGrpcWebRequestsStarted   atomic.Int64
		TCPOtherRequestsStarted     atomic.Int64
		TCPGrpcRequestsCompleted    atomic.Int64
		TCPGrpcWebRequestsCompleted atomic.Int64
		TCPOtherRequestsCompleted   atomic.Int64
	}
}

var errMixedUnauthAndAuth = errors.New("cannot use unauthenticated and auth handlers at same time")

func addrsForInterface(iface *net.Interface) ([]string, []string) {
	var v4, v6, v6local []string
	addrs, err := iface.Addrs()
	if err != nil {
		return v4, v6
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				v4 = append(v4, ipnet.IP.String())
			} else {
				switch ip := ipnet.IP.To16(); ip != nil {
				case ip.IsGlobalUnicast():
					v6 = append(v6, ipnet.IP.String())
				case ip.IsLinkLocalUnicast():
					v6local = append(v6local, ipnet.IP.String())
				}
			}
		}
	}
	if len(v6) == 0 {
		v6 = v6local
	}
	return v4, v6
}

// NewServer returns a new server ready to be started that
// will listen on localhost on a random port unless TLS is turned
// on and authentication is enabled in which case the server will
// listen on all interfaces.
func NewServer(logger utils.ZapCompatibleLogger, opts ...ServerOption) (Server, error) {
	var sOpts serverOptions
	for _, opt := range opts {
		if err := opt.apply(&sOpts); err != nil {
			return nil, err
		}
	}
	if sOpts.unauthenticated && (len(sOpts.authHandlersForCreds) != 0 || sOpts.tlsAuthHandler != nil) {
		return nil, errMixedUnauthAndAuth
	}

	grpcBindAddr := sOpts.bindAddress
	if grpcBindAddr == "" {
		if sOpts.tlsConfig == nil || sOpts.unauthenticated {
			grpcBindAddr = "localhost:0"
		} else {
			grpcBindAddr = ":0"
		}
	}

	grpcListener, err := net.Listen("tcp", grpcBindAddr)
	if err != nil {
		return nil, err
	}

	serverOpts := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             keepAliveTime / 2, // a little extra buffer to try to avoid ENHANCE_YOUR_CALM
			PermitWithoutStream: true,
		}),
	}

	var firstSeenTLSCert *tls.Certificate
	if sOpts.tlsConfig != nil {
		if len(sOpts.tlsConfig.Certificates) == 0 {
			if cert, err := sOpts.tlsConfig.GetCertificate(&tls.ClientHelloInfo{}); err == nil {
				firstSeenTLSCert = cert
			} else {
				return nil, errors.New("invalid *tls.Config; expected at least 1 certificate")
			}
		} else {
			firstSeenTLSCert = &sOpts.tlsConfig.Certificates[0]
		}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(sOpts.tlsConfig)))
	}

	var firstSeenTLSCertLeaf *x509.Certificate
	if firstSeenTLSCert != nil {
		leaf, err := x509.ParseCertificate(firstSeenTLSCert.Certificate[0])
		if err != nil {
			return nil, err
		}
		firstSeenTLSCertLeaf = leaf
	}

	httpServer := &http.Server{
		ReadTimeout:    10 * time.Second,
		MaxHeaderBytes: MaxMessageSize,
	}

	if len(sOpts.authKeys) == 0 {
		if sOpts.jwtSignerKeyID != "" {
			return nil, errors.New("cannot use WithJWTSignerKeyID if no auth keys are set")
		}
		if !sOpts.unauthenticated {
			_, privKey, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				return nil, err
			}
			pubKey := privKey.Public()
			keyID := ED25519PublicKeyThumbprint(pubKey.(ed25519.PublicKey))
			data := authKeyData{
				id:         keyID,
				method:     jwt.SigningMethodEdDSA,
				privateKey: privKey,
				publicKey:  pubKey,
			}
			sOpts.authKeys = map[string]authKeyData{
				keyID: data,
			}
			sOpts.jwtSignerKeyID = keyID
		}
	} else if sOpts.jwtSignerKeyID == "" {
		if len(sOpts.authKeys) > 1 {
			return nil, errors.New("must use WithJWTSignerKeyID if more than one private key in use")
		}
		for _, first := range sOpts.authKeys {
			if first.id == "" {
				return nil, errors.New("invariant: auth key has no id")
			}
			sOpts.jwtSignerKeyID = first.id
			break
		}
	}

	// double check we set everything up correctly via options
	for _, data := range sOpts.authKeys {
		if err := data.Validate(); err != nil {
			return nil, err
		}
	}

	var authKeyForJWTSigning authKeyData
	if !sOpts.unauthenticated {
		var ok bool
		authKeyForJWTSigning, ok = sOpts.authKeys[sOpts.jwtSignerKeyID]
		if !ok {
			return nil, fmt.Errorf("no auth key data set for key id %q", sOpts.jwtSignerKeyID)
		}
	}

	internalCredsKey := make([]byte, 64)
	_, err = rand.Read(internalCredsKey)
	if err != nil {
		return nil, err
	}

	if sOpts.authHandlersForCreds == nil {
		sOpts.authHandlersForCreds = make(map[CredentialsType]credAuthHandlers)
	}

	grpcGatewayHandler := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
	)

	server := &simpleServer{
		grpcListener:         grpcListener,
		httpServer:           httpServer,
		grpcGatewayHandler:   grpcGatewayHandler,
		authKeys:             sOpts.authKeys,
		authKeyForJWTSigning: authKeyForJWTSigning,
		internalUUID:         uuid.NewString(),
		internalCreds: Credentials{
			Type:    credentialsTypeInternal,
			Payload: base64.StdEncoding.EncodeToString(internalCredsKey),
		},
		tlsAuthHandler:       sOpts.tlsAuthHandler,
		authHandlersForCreds: sOpts.authHandlersForCreds,
		authToHandler:        sOpts.authToHandler,
		authAudience:         sOpts.authAudience,
		authIssuer:           sOpts.authIssuer,
		ensureAuthedHandler:  sOpts.ensureAuthedHandler,
		exemptMethods:        make(map[string]bool),
		publicMethods:        make(map[string]bool),
		tlsConfig:            sOpts.tlsConfig,
		firstSeenTLSCertLeaf: firstSeenTLSCertLeaf,
		logger:               logger,
	}

	if sOpts.unknownStreamDesc != nil {
		serverOpts = append(serverOpts, grpc.UnknownServiceHandler(sOpts.unknownStreamDesc.Handler))
	}
	var unaryInterceptors []grpc.UnaryServerInterceptor
	unaryInterceptors = append(unaryInterceptors,
		grpc_recovery.UnaryServerInterceptor(grpc_recovery.WithRecoveryHandler(
			grpc_recovery.RecoveryHandlerFunc(func(p interface{}) error {
				err := status.Errorf(codes.Internal, "%v", p)
				logger.Errorw("panicked while calling unary server method", "error", errors.WithStack(err))
				return err
			}))),
		grpcUnaryServerInterceptor(logger),
		unaryServerCodeInterceptor(),
	)
	unaryInterceptors = append(unaryInterceptors, UnaryServerTracingInterceptor())
	unaryAuthIntPos := -1
	if !sOpts.unauthenticated {
		unaryInterceptors = append(unaryInterceptors, server.authUnaryInterceptor)
		unaryAuthIntPos = len(unaryInterceptors) - 1
	}
	if sOpts.unaryInterceptor != nil {
		unaryInterceptors = append(unaryInterceptors, func(
			ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (interface{}, error) {
			if server.exemptMethods[info.FullMethod] {
				return handler(ctx, req)
			}
			return sOpts.unaryInterceptor(ctx, req, info, handler)
		})
	}
	unaryInterceptor := grpc_middleware.ChainUnaryServer(unaryInterceptors...)
	serverOpts = append(serverOpts, grpc.UnaryInterceptor(unaryInterceptor))

	var streamInterceptors []grpc.StreamServerInterceptor
	streamInterceptors = append(streamInterceptors,
		grpc_recovery.StreamServerInterceptor(grpc_recovery.WithRecoveryHandler(
			grpc_recovery.RecoveryHandlerFunc(func(p interface{}) error {
				err := status.Errorf(codes.Internal, "%s", p)
				logger.Errorw("panicked while calling stream server method", "error", errors.WithStack(err))
				return err
			}))),
		grpcStreamServerInterceptor(logger),
		streamServerCodeInterceptor(),
	)
	streamInterceptors = append(streamInterceptors, StreamServerTracingInterceptor())
	streamAuthIntPos := -1
	if !sOpts.unauthenticated {
		streamInterceptors = append(streamInterceptors, server.authStreamInterceptor)
		streamAuthIntPos = len(streamInterceptors) - 1
	}
	if sOpts.streamInterceptor != nil {
		streamInterceptors = append(streamInterceptors, func(
			srv interface{},
			serverStream grpc.ServerStream,
			info *grpc.StreamServerInfo,
			handler grpc.StreamHandler,
		) error {
			if server.exemptMethods[info.FullMethod] {
				return handler(srv, serverStream)
			}
			return sOpts.streamInterceptor(srv, serverStream, info, handler)
		})
	}
	streamInterceptor := grpc_middleware.ChainStreamServer(streamInterceptors...)
	serverOpts = append(serverOpts, grpc.StreamInterceptor(streamInterceptor))

	if sOpts.statsHandler != nil {
		serverOpts = append(serverOpts, grpc.StatsHandler(sOpts.statsHandler))
	}

	serverOpts = append(serverOpts, grpc.WaitForHandlers(true))

	grpcServer := grpc.NewServer(
		serverOpts...,
	)
	reflection.Register(grpcServer)
	grpcWebServer := grpcweb.WrapServer(grpcServer, grpcweb.WithOriginFunc(func(origin string) bool {
		return true
	}))

	server.grpcServer = grpcServer
	server.grpcWebServer = grpcWebServer

	if !sOpts.unauthenticated {
		if err := server.RegisterServiceServer(
			context.Background(),
			&rpcpb.AuthService_ServiceDesc,
			server,
			rpcpb.RegisterAuthServiceHandlerFromEndpoint,
		); err != nil {
			return nil, err
		}
		server.authHandlersForCreds[credentialsTypeInternal] = credAuthHandlers{
			AuthHandler: MakeSimpleAuthHandler(
				[]string{server.internalUUID}, server.internalCreds.Payload),
		}
		// Update this if the proto method or path changes
		server.exemptMethods["/proto.rpc.v1.AuthService/Authenticate"] = true
	}

	if sOpts.allowUnauthenticatedHealthCheck {
		server.exemptMethods[healthCheckMethod] = true
		server.exemptMethods[healthWatchMethod] = true
	}

	for _, method := range sOpts.publicMethods {
		server.publicMethods[method] = true
	}

	if sOpts.authToHandler != nil {
		if err := server.RegisterServiceServer(
			context.Background(),
			&rpcpb.ExternalAuthService_ServiceDesc,
			server,
			rpcpb.RegisterExternalAuthServiceHandlerFromEndpoint,
		); err != nil {
			return nil, err
		}
	}

	var mDNSAddress *net.TCPAddr
	if sOpts.listenerAddress != nil {
		mDNSAddress = sOpts.listenerAddress
	} else {
		var ok bool
		mDNSAddress, ok = grpcListener.Addr().(*net.TCPAddr)
		if !ok {
			return nil, errors.Errorf("expected *net.TCPAddr but got %T", grpcListener.Addr())
		}
	}

	supportedServices := []string{"grpc"}
	if sOpts.webrtcOpts.Enable {
		supportedServices = append(supportedServices, "webrtc")
	}
	instanceNames := sOpts.instanceNames
	if len(instanceNames) == 0 {
		instanceName, err := InstanceNameFromAddress(mDNSAddress.String())
		if err != nil {
			return nil, err
		}
		instanceNames = []string{instanceName}
	}
	server.instanceNames = instanceNames

	if len(server.authAudience) == 0 {
		logger.Debugw("auth audience unset; using instance names instead", "auth_audience", server.instanceNames)
		server.authAudience = server.instanceNames
	}

	if server.authIssuer == "" {
		logger.Debugw("auth issuer unset; using first auth audience member instead", "auth_issuer", server.authAudience[0])
		server.authIssuer = server.authAudience[0]
	}

	if !sOpts.disableMDNS {
		if mDNSAddress.IP.IsLoopback() {
			hostname, err := os.Hostname()
			if err != nil {
				return nil, err
			}
			ifcs, err := net.Interfaces()
			if err != nil {
				return nil, err
			}
			var loopbackIfaces []net.Interface
			for _, ifc := range ifcs {
				if (ifc.Flags&net.FlagUp) == 0 || (ifc.Flags&net.FlagLoopback) == 0 {
					continue
				}
				loopbackIfaces = append(loopbackIfaces, ifc)
				break
			}
			for _, host := range instanceNames {
				hosts := []string{host, strings.ReplaceAll(host, ".", "-")}
				for _, host := range hosts {
					mdnsServer, err := zeroconf.RegisterProxy(
						host,
						"_rpc._tcp",
						"local.",
						mDNSAddress.Port,
						hostname,
						[]string{"127.0.0.1"},
						supportedServices,
						loopbackIfaces,
						// RSDK-8205: logger.Desugar().Sugar() is necessary to massage a ZapCompatibleLogger into a
						// *zap.SugaredLogger to match zeroconf function signatures.
						logger.Desugar().Sugar(),
					)
					if err != nil {
						logger.Warnw(mDNSerr, "error", err)
						sOpts.disableMDNS = true
						break
					}
					server.mdnsServers = append(server.mdnsServers, mdnsServer)

					// register a second address to match queries for machine-name.local
					// RSDK-10409 - Depending on if we need the previous block to register mDNS addresses with
					// the system hostname, we may be able to combine the two separate registrations into one.
					if host != hostname {
						mdnsServer, err = zeroconf.RegisterProxy(
							host,
							"_rpc._tcp",
							"local.",
							mDNSAddress.Port,
							host,
							[]string{"127.0.0.1"},
							supportedServices,
							loopbackIfaces,
							// RSDK-8205: logger.Desugar().Sugar() is necessary to massage a ZapCompatibleLogger into a
							// *zap.SugaredLogger to match zeroconf function signatures.
							logger.Desugar().Sugar(),
						)
						if err != nil {
							logger.Warnw(mDNSerr, "error", err)
							sOpts.disableMDNS = true
							break
						}
						server.mdnsServers = append(server.mdnsServers, mdnsServer)
					}
				}
			}
		} else {
			for _, host := range instanceNames {
				hosts := []string{host, strings.ReplaceAll(host, ".", "-")}

				// all of this mimics code in zeroconf.Register, with the change of
				// using the host as hostname instead of os.Hostname.
				ifaces := listMulticastInterfaces()
				addrV4 := make([]string, 0)
				addrV6 := make([]string, 0)
				for _, iface := range ifaces {
					v4, v6 := addrsForInterface(&iface)
					addrV4 = append(addrV4, v4...)
					addrV6 = append(addrV6, v6...)
				}
				for _, host := range hosts {
					mdnsServer, err := zeroconf.RegisterDynamic(
						host,
						"_rpc._tcp",
						"local.",
						mDNSAddress.Port,
						supportedServices,
						nil,
						// RSDK-8205: logger.Desugar().Sugar() is necessary to massage a ZapCompatibleLogger into a
						// *zap.SugaredLogger to match zeroconf function signatures.
						logger.Desugar().Sugar(),
					)
					if err != nil {
						logger.Warnw(mDNSerr, "error", err)
						sOpts.disableMDNS = true
						break
					}
					server.mdnsServers = append(server.mdnsServers, mdnsServer)

					// register a second address to match queries for machine-name.local
					// RSDK-10409 - Depending on if we need the previous block to register mDNS addresses with
					// the system hostname, we may be able to combine the two separate registrations into one.
					hostname, err := os.Hostname()
					if err == nil && host == hostname {
						continue
					}

					mdnsServer, err = zeroconf.RegisterProxy(
						host,
						"_rpc._tcp",
						"local.",
						mDNSAddress.Port,
						host,
						append(addrV4, addrV6...),
						supportedServices,
						ifaces,
						// RSDK-8205: logger.Desugar().Sugar() is necessary to massage a ZapCompatibleLogger into a
						// *zap.SugaredLogger to match zeroconf function signatures.
						logger.Desugar().Sugar(),
					)
					if err != nil {
						logger.Warnw(mDNSerr, "error", err)
						sOpts.disableMDNS = true
						break
					}
					server.mdnsServers = append(server.mdnsServers, mdnsServer)
				}
			}
		}
	}

	if sOpts.webrtcOpts.Enable {
		// TODO(GOUT-11): Handle auth; right now we assume
		// successful auth to the signaler implies that auth should be allowed here, which is not 100%
		// true.
		webrtcUnaryInterceptors := make([]grpc.UnaryServerInterceptor, 0, len(unaryInterceptors))
		webrtcStreamInterceptors := make([]grpc.StreamServerInterceptor, 0, len(streamInterceptors))
		for idx, interceptor := range unaryInterceptors {
			if idx == unaryAuthIntPos {
				continue
			}
			webrtcUnaryInterceptors = append(webrtcUnaryInterceptors, interceptor)
		}
		for idx, interceptor := range streamInterceptors {
			if idx == streamAuthIntPos {
				continue
			}
			webrtcStreamInterceptors = append(webrtcStreamInterceptors, interceptor)
		}
		unaryInterceptor := grpc_middleware.ChainUnaryServer(webrtcUnaryInterceptors...)
		streamInterceptor := grpc_middleware.ChainStreamServer(webrtcStreamInterceptors...)

		if sOpts.unknownStreamDesc == nil {
			server.webrtcServer = newWebRTCServerWithInterceptors(
				logger,
				unaryInterceptor,
				streamInterceptor,
			)
		} else {
			server.webrtcServer = newWebRTCServerWithInterceptorsAndUnknownStreamHandler(
				logger,
				unaryInterceptor,
				streamInterceptor,
				sOpts.unknownStreamDesc,
			)
		}
		reflection.Register(server.webrtcServer)

		config := DefaultWebRTCConfiguration
		if sOpts.webrtcOpts.Config != nil {
			config = *sOpts.webrtcOpts.Config
		}

		externalSignalingHosts := sOpts.webrtcOpts.ExternalSignalingHosts
		internalSignalingHosts := sOpts.webrtcOpts.InternalSignalingHosts
		if len(externalSignalingHosts) == 0 {
			externalSignalingHosts = instanceNames
		}
		if len(internalSignalingHosts) == 0 {
			internalSignalingHosts = instanceNames
		}

		if sOpts.webrtcOpts.ExternalSignalingAddress != "" {
			logger.Infow(
				"Running external signaling",
				"signaling_address", sOpts.webrtcOpts.ExternalSignalingAddress,
				"for_hosts", externalSignalingHosts,
			)
			server.webrtcAnswerers = append(server.webrtcAnswerers, newWebRTCSignalingAnswerer(
				sOpts.webrtcOpts.ExternalSignalingAddress,
				externalSignalingHosts,
				server.webrtcServer,
				sOpts.webrtcOpts.ExternalSignalingDialOpts,
				config,
				utils.Sublogger(logger, "signaler.external"),
			))
		} else {
			sOpts.webrtcOpts.EnableInternalSignaling = true
		}

		if sOpts.webrtcOpts.EnableInternalSignaling {
			signalingCallQueue := NewMemoryWebRTCCallQueue(logger)
			server.signalingCallQueue = signalingCallQueue
			server.signalingServer = NewWebRTCSignalingServer(signalingCallQueue, nil, logger,
				defaultHeartbeatInterval, internalSignalingHosts...)
			if err := server.RegisterServiceServer(
				context.Background(),
				&webrtcpb.SignalingService_ServiceDesc,
				server.signalingServer,
				webrtcpb.RegisterSignalingServiceHandlerFromEndpoint,
			); err != nil {
				return nil, err
			}

			address := grpcListener.Addr().String()
			logger.Infow(
				"Running internal signaling",
				"signaling_address", address,
				"for_hosts", internalSignalingHosts,
			)
			var answererDialOpts []DialOption
			if sOpts.tlsConfig != nil {
				tlsConfig := sOpts.tlsConfig.Clone()
				tlsConfig.ServerName = server.firstSeenTLSCertLeaf.Subject.CommonName
				answererDialOpts = append(answererDialOpts, WithTLSConfig(tlsConfig))
			} else {
				answererDialOpts = append(answererDialOpts, WithInsecure())
			}
			if !sOpts.unauthenticated {
				answererDialOpts = append(answererDialOpts, WithEntityCredentials(server.internalUUID, server.internalCreds))
			}
			// this answerer uses an internal signaling server that runs locally as a separate process and so does not get a shared
			// connection to App as a dial option
			server.webrtcAnswerers = append(server.webrtcAnswerers, newWebRTCSignalingAnswerer(
				address,
				internalSignalingHosts,
				server.webrtcServer,
				answererDialOpts,
				config,
				utils.Sublogger(logger, "signaler.internal"),
			))
		}
	}

	return server, nil
}

func (ss *simpleServer) InstanceNames() []string {
	return ss.instanceNames
}

type requestType int

const (
	requestTypeNone requestType = iota
	requestTypeGRPC
	requestTypeGRPCWeb
)

func (ss *simpleServer) getRequestType(r *http.Request) requestType {
	if ss.grpcWebServer.IsAcceptableGrpcCorsRequest(r) || ss.grpcWebServer.IsGrpcWebRequest(r) {
		return requestTypeGRPCWeb
	} else if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
		return requestTypeGRPC
	}
	return requestTypeNone
}

func requestWithHost(r *http.Request) *http.Request {
	if r.Host == "" {
		return r
	}
	host := strings.Split(r.Host, ":")[0]
	return r.WithContext(contextWithHost(r.Context(), host))
}

func (ss *simpleServer) GatewayHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ss.grpcGatewayHandler.ServeHTTP(w, requestWithHost(r))
	})
}

func (ss *simpleServer) GRPCHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = requestWithHost(r)
		switch ss.getRequestType(r) {
		case requestTypeGRPC:
			ss.grpcServer.ServeHTTP(w, r)
		case requestTypeGRPCWeb:
			ss.grpcWebServer.ServeHTTP(w, r)
		case requestTypeNone:
			fallthrough
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
}

func (ss *simpleServer) EnsureAuthed(ctx context.Context) (context.Context, error) {
	return ss.ensureAuthed(ctx)
}

// ServeHTTP is an all-in-one handler for any kind of gRPC traffic. This is useful
// in a scenario where all gRPC is served from the root path due to limitations of normal
// gRPC being served from a non-root path.
func (ss *simpleServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = requestWithHost(r)
	switch ss.getRequestType(r) {
	case requestTypeGRPC:
		ss.counters.TCPGrpcRequestsStarted.Add(1)
		ss.grpcServer.ServeHTTP(w, r)
		ss.counters.TCPGrpcRequestsCompleted.Add(1)
	case requestTypeGRPCWeb:
		ss.counters.TCPGrpcWebRequestsStarted.Add(1)
		ss.grpcWebServer.ServeHTTP(w, r)
		ss.counters.TCPGrpcWebRequestsCompleted.Add(1)
	case requestTypeNone:
		fallthrough
	default:
		ss.counters.TCPOtherRequestsStarted.Add(1)
		ss.grpcGatewayHandler.ServeHTTP(w, r)
		ss.counters.TCPOtherRequestsCompleted.Add(1)
	}
}

func (ss *simpleServer) InternalAddr() net.Addr {
	return ss.grpcListener.Addr()
}

func (ss *simpleServer) Start() error {
	ss.mu.Lock()
	if ss.stopped {
		ss.mu.Unlock()
		return errors.New("server stopped")
	}
	ss.mu.Unlock()

	var err error
	var errMu sync.Mutex
	utils.PanicCapturingGo(func() {
		if serveErr := ss.grpcServer.Serve(ss.grpcListener); serveErr != nil {
			errMu.Lock()
			err = multierr.Combine(err, serveErr)
			errMu.Unlock()
		}
	})

	for _, answerer := range ss.webrtcAnswerers {
		answerer.Start()
	}

	errMu.Lock()
	defer errMu.Unlock()
	return err
}

func (ss *simpleServer) Serve(listener net.Listener) error {
	return ss.serveTLS(listener, "", "", nil)
}

func (ss *simpleServer) ServeTLS(listener net.Listener, certFile, keyFile string, tlsConfig *tls.Config) error {
	return ss.serveTLS(listener, certFile, keyFile, tlsConfig)
}

func (ss *simpleServer) serveTLS(listener net.Listener, certFile, keyFile string, tlsConfig *tls.Config) error {
	ss.mu.Lock()
	if ss.stopped {
		ss.mu.Unlock()
		return errors.New("server stopped")
	}
	ss.httpServer.Addr = listener.Addr().String()
	ss.httpServer.Handler = ss
	secure := true
	if certFile == "" && keyFile == "" {
		secure = false
		http2Server, err := utils.NewHTTP2Server()
		if err != nil {
			return err
		}
		ss.httpServer.RegisterOnShutdown(func() {
			utils.UncheckedErrorFunc(http2Server.Close)
		})
		ss.httpServer.Handler = h2c.NewHandler(ss.httpServer.Handler, http2Server.HTTP2)
	}

	var err error
	var errMu sync.Mutex
	ss.activeBackgroundWorkers.Add(1)
	ss.mu.Unlock()
	utils.ManagedGo(func() {
		var serveErr error
		if secure {
			if tlsConfig != nil {
				ss.httpServer.TLSConfig = tlsConfig.Clone()
			}
			serveErr = ss.httpServer.ServeTLS(listener, certFile, keyFile)
		} else {
			serveErr = ss.httpServer.Serve(listener)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errMu.Lock()
			err = multierr.Combine(err, serveErr)
			errMu.Unlock()
		}
	}, ss.activeBackgroundWorkers.Done)
	startErr := ss.Start()
	errMu.Lock()
	err = multierr.Combine(err, startErr)
	errMu.Unlock()
	return err
}

func (ss *simpleServer) Stop() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.stopped {
		return nil
	}
	ss.stopped = true
	var err error
	ss.logger.Info("stopping")
	for idx, answerer := range ss.webrtcAnswerers {
		ss.logger.Debugw("stopping WebRTC answerer", "num", idx)
		answerer.Stop()
		ss.logger.Debugw("WebRTC answerer stopped", "num", idx)
	}
	if ss.signalingServer != nil {
		ss.signalingServer.Close()
	}
	if ss.signalingCallQueue != nil {
		err = multierr.Combine(err, ss.signalingCallQueue.Close())
	}
	ss.logger.Debug("stopping gRPC server")
	defer ss.grpcServer.Stop()
	ss.logger.Debug("canceling service servers for gateway")
	for _, cancel := range ss.serviceServerCancels {
		cancel()
	}
	ss.logger.Debug("service servers for gateway canceled")
	if ss.webrtcServer != nil {
		ss.logger.Debug("stopping WebRTC server")
		ss.webrtcServer.Stop()
		ss.logger.Debug("WebRTC server stopped")
	}
	for idx, mdnsServer := range ss.mdnsServers {
		ss.logger.Debugf("shutting down mDNS server %d of %d", idx+1, len(ss.mdnsServers))
		mdnsServer.Shutdown()
	}
	ss.logger.Debug("shutting down HTTP server")
	err = multierr.Combine(err, ss.httpServer.Shutdown(context.Background()))
	ss.logger.Debug("HTTP server shut down")
	ss.activeBackgroundWorkers.Wait()
	ss.logger.Info("stopped cleanly")
	return err
}

// SimpleServerStats are stats of the simple variety.
type SimpleServerStats struct {
	TCPGrpcStats    TCPGrpcStats
	WebRTCGrpcStats WebRTCGrpcStats
}

// TCPGrpcStats are stats for the classic tcp/http2 webserver.
type TCPGrpcStats struct {
	RequestsStarted        int64
	WebRequestsStarted     int64
	OtherRequestsStarted   int64
	RequestsCompleted      int64
	WebRequestsCompleted   int64
	OtherRequestsCompleted int64
}

// Stats returns stats. The return value of `any` is to satisfy the FTDC interface.
func (ss *simpleServer) Stats() any {
	return SimpleServerStats{
		TCPGrpcStats: TCPGrpcStats{
			RequestsStarted:        ss.counters.TCPGrpcRequestsStarted.Load(),
			WebRequestsStarted:     ss.counters.TCPGrpcWebRequestsStarted.Load(),
			OtherRequestsStarted:   ss.counters.TCPOtherRequestsStarted.Load(),
			RequestsCompleted:      ss.counters.TCPGrpcRequestsCompleted.Load(),
			WebRequestsCompleted:   ss.counters.TCPGrpcWebRequestsCompleted.Load(),
			OtherRequestsCompleted: ss.counters.TCPOtherRequestsCompleted.Load(),
		},
		WebRTCGrpcStats: ss.webrtcServer.Stats(),
	}
}

// A RegisterServiceHandlerFromEndpointFunc is a means to have a service attach itself to a gRPC gateway mux.
type RegisterServiceHandlerFromEndpointFunc func(
	ctx context.Context,
	mux *runtime.ServeMux,
	endpoint string,
	opts []grpc.DialOption,
) (err error)

func (ss *simpleServer) RegisterServiceServer(
	ctx context.Context,
	svcDesc *grpc.ServiceDesc,
	svcServer interface{},
	svcHandlers ...RegisterServiceHandlerFromEndpointFunc,
) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	stopCtx, stopCancel := context.WithCancel(ctx)
	ss.serviceServerCancels = append(ss.serviceServerCancels, stopCancel)
	ss.grpcServer.RegisterService(svcDesc, svcServer)
	if ss.webrtcServer != nil {
		//nolint:contextcheck
		ss.webrtcServer.RegisterService(svcDesc, svcServer)
	}
	if len(svcHandlers) != 0 {
		addr := ss.grpcListener.Addr().String()
		opts := []grpc.DialOption{grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize))}
		if ss.tlsConfig == nil {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			tlsConfig := ss.tlsConfig.Clone()
			tlsConfig.ServerName = ss.firstSeenTLSCertLeaf.DNSNames[0]
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		}
		for _, h := range svcHandlers {
			if err := h(stopCtx, ss.grpcGatewayHandler, addr, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

func unaryServerCodeInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		if s := status.FromContextError(err); s != nil {
			return nil, s.Err()
		}
		return nil, err
	}
}

func streamServerCodeInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, stream)
		if err == nil {
			return nil
		}
		if _, ok := status.FromError(err); ok {
			return err
		}
		if s := status.FromContextError(err); s != nil {
			return s.Err()
		}
		return err
	}
}

// InstanceNameFromAddress returns a suitable instance name given an address.
// If it's empty or an IP address, a new UUID is returned.
func InstanceNameFromAddress(addr string) (string, error) {
	if strings.Contains(addr, ":") {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return "", err
		}
		addr = host
	}
	if net.ParseIP(addr) == nil {
		return addr, nil
	}
	// will use a UUID since we have no better choice
	return uuid.NewString(), nil
}

// PeerConnectionType describes the type of connection of a peer.
type PeerConnectionType uint16

// Known types of peer connections.
const (
	PeerConnectionTypeUnknown = PeerConnectionType(iota)
	PeerConnectionTypeGRPC
	PeerConnectionTypeWebRTC
)

// PeerConnectionInfo details information about a connection.
type PeerConnectionInfo struct {
	ConnectionType PeerConnectionType
	LocalAddress   string
	RemoteAddress  string
}

// PeerConnectionInfoFromContext returns as much information about the connection as can be found
// from the request context.
func PeerConnectionInfoFromContext(ctx context.Context) PeerConnectionInfo {
	if p, ok := peer.FromContext(ctx); ok && p != nil {
		return PeerConnectionInfo{
			ConnectionType: PeerConnectionTypeGRPC,
			RemoteAddress:  p.Addr.String(),
		}
	}
	if pc, ok := ContextPeerConnection(ctx); ok {
		candPair, hasCandPair := webrtcPeerConnCandPair(pc)
		if hasCandPair {
			return PeerConnectionInfo{
				ConnectionType: PeerConnectionTypeWebRTC,
				LocalAddress:   candPair.Local.String(),
				RemoteAddress:  candPair.Remote.String(),
			}
		}
	}
	return PeerConnectionInfo{
		ConnectionType: PeerConnectionTypeUnknown,
	}
}
