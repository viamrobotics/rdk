// Package web provides gRPC/REST/GUI APIs to control and monitor a robot.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	"goji.io"
	"goji.io/pat"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	grpcserver "go.viam.com/rdk/robot/server"
	weboptions "go.viam.com/rdk/robot/web/options"
	webstream "go.viam.com/rdk/robot/web/stream"
	rutils "go.viam.com/rdk/utils"
)

// SubtypeName is a constant that identifies the internal web resource subtype string.
const (
	SubtypeName = "web"
	// TCPParentPort is the port of the parent socket when VIAM_TCP_MODE is set.
	TCPParentPort = 0
)

// API is the fully qualified API for the internal web service.
var API = resource.APINamespaceRDKInternal.WithServiceType(SubtypeName)

// InternalServiceName is used to refer to/depend on this service internally.
var InternalServiceName = resource.NewName(API, "builtin")

// A Service controls the web server for a robot.
type Service interface {
	resource.Resource

	// Start starts the web server
	Start(context.Context, weboptions.Options) error

	// Stop stops the main web service (but leaves module server socket running.)
	Stop()

	// StartModule starts the module server socket.
	StartModule(context.Context) error

	// Returns the address and port the web service listens on.
	Address() string

	// Returns the unix socket path the module server listens on.
	ModuleAddresses() config.ParentSockAddrs

	Stats() any

	RequestCounter() *RequestCounter

	ModPeerConnTracker() *grpc.ModPeerConnTracker
}

// resourceGetterForAPI is a type adapter that allows [robot.LocalRobot] to be used
// as a [resource.APIResourceGetter] for a provided [resource.API] type.
type resourceGetterForAPI struct {
	api   resource.API
	robot robot.LocalRobot
}

func (r resourceGetterForAPI) Resource(name string) (resource.Resource, error) {
	return r.robot.FindBySimpleNameAndAPI(name, r.api)
}

type webService struct {
	resource.Named

	mu            sync.Mutex
	r             robot.LocalRobot
	rpcServer     rpc.Server
	unixModServer rpc.Server
	tcpModServer  rpc.Server

	// Will be nil on non-cgo builds.
	streamServer *webstream.Server
	opts         options
	addr         string
	modAddrs     config.ParentSockAddrs
	logger       logging.Logger
	cancelCtx    context.Context
	cancelFunc   func()
	isRunning    bool
	webWorkers   sync.WaitGroup
	modWorkers   sync.WaitGroup

	requestCounter     RequestCounter
	modPeerConnTracker *grpc.ModPeerConnTracker
}

// New returns a new web service for the given robot.
func New(r robot.LocalRobot, logger logging.Logger, opts ...Option) Service {
	var wOpts options
	for _, opt := range opts {
		opt.apply(&wOpts)
	}
	webSvc := &webService{
		Named:              InternalServiceName.AsNamed(),
		r:                  r,
		logger:             logger,
		rpcServer:          nil,
		streamServer:       nil,
		modPeerConnTracker: grpc.NewModPeerConnTracker(),
		opts:               wOpts,
		requestCounter:     RequestCounter{logger: logger},
	}
	webSvc.requestCounter.ensureLimit()
	return webSvc
}

var internalWebServiceName = resource.NewName(
	resource.APINamespaceRDKInternal.WithServiceType("web"),
	"builtin",
)

func (svc *webService) Name() resource.Name {
	return internalWebServiceName
}

// Start starts the web server, will return an error if server is already up.
func (svc *webService) Start(ctx context.Context, o weboptions.Options) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.isRunning {
		return errors.New("web server already started")
	}
	svc.isRunning = true
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	svc.cancelCtx = cancelCtx
	svc.cancelFunc = cancelFunc

	if err := svc.runWeb(svc.cancelCtx, o); err != nil {
		if svc.cancelFunc != nil {
			svc.cancelFunc()
		}
		svc.isRunning = false
		return err
	}
	return nil
}

// RunWeb starts the web server on the robot with web options and blocks until we cancel the context.
func RunWeb(ctx context.Context, r robot.LocalRobot, o weboptions.Options, logger logging.Logger) (err error) {
	defer func() {
		if err != nil {
			err = utils.FilterOutError(err, context.Canceled)
			if err != nil {
				logger.Errorw("error running web", "error", err)
			}
		}
	}()

	if err := r.StartWeb(ctx, o); err != nil {
		return err
	}
	<-ctx.Done()
	logger.Info("Viam RDK shutting down")
	return ctx.Err()
}

// RunWebWithConfig starts the web server on the robot with a robot config and blocks until we cancel the context.
func RunWebWithConfig(ctx context.Context, r robot.LocalRobot, cfg *config.Config, logger logging.Logger) error {
	o, err := weboptions.FromConfig(cfg)
	if err != nil {
		return err
	}
	return RunWeb(ctx, r, o, logger)
}

// Address returns the address the service is listening on.
func (svc *webService) Address() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.addr
}

// ModuleAddress returns the unix socket path the module server is listening on.
func (svc *webService) ModuleAddresses() config.ParentSockAddrs {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.modAddrs
}

// returns (listener, addr, error).
func (svc *webService) startProtocolModuleParentServer(ctx context.Context, tcpMode bool) error {
	if tcpMode && svc.tcpModServer != nil || !tcpMode && svc.unixModServer != nil {
		return errors.New("module service already started")
	}

	var lis net.Listener
	var addr string
	if err := module.MakeSelfOwnedFilesFunc(func() error {
		dir, err := rutils.PlatformMkdirTemp("", "viam-module-*")
		if err != nil {
			return errors.WithMessage(err, "module startup failed")
		}

		if tcpMode {
			addr = "127.0.0.1:" + strconv.Itoa(TCPParentPort)
			lis, err = net.Listen("tcp", addr)
		} else {
			addr, err = module.CreateSocketAddress(dir, "parent")
			if err != nil {
				return errors.WithMessage(err, "module startup failed")
			}
			lis, err = net.Listen("unix", addr)
		}
		if err != nil {
			return errors.WithMessage(err, "failed to listen")
		}
		if tcpMode {
			svc.modAddrs.TCPAddr = lis.Addr().String()
		} else {
			svc.modAddrs.UnixAddr = addr
		}
		return nil
	}); err != nil {
		return err
	}
	var (
		unaryInterceptors  []googlegrpc.UnaryServerInterceptor
		streamInterceptors []googlegrpc.StreamServerInterceptor
	)

	unaryInterceptors = append(unaryInterceptors, grpc.EnsureTimeoutUnaryServerInterceptor)

	// Attach the module name (as defined by the robot config) to the handler context. Can be
	// accessed via `grpc.GetModuleName`.
	unaryInterceptors = append(unaryInterceptors, svc.modPeerConnTracker.ModInfoUnaryServerInterceptor)

	unaryInterceptors = append(unaryInterceptors, svc.requestCounter.UnaryInterceptor)
	streamInterceptors = append(streamInterceptors, svc.requestCounter.StreamInterceptor)

	// Add recovery handler interceptors to avoid crashing the rdk when a module's gRPC
	// request manages to cause an internal panic.
	unaryInterceptors = append(unaryInterceptors, grpc_recovery.UnaryServerInterceptor(grpc_recovery.WithRecoveryHandler(
		grpc_recovery.RecoveryHandlerFunc(func(p interface{}) error {
			err := status.Errorf(codes.Internal, "%v", p)
			svc.logger.Errorw("panicked while calling unary server method for module request", "error", errors.WithStack(err))
			return err
		}))))
	streamInterceptors = append(streamInterceptors, grpc_recovery.StreamServerInterceptor(grpc_recovery.WithRecoveryHandler(
		grpc_recovery.RecoveryHandlerFunc(func(p interface{}) error {
			err := status.Errorf(codes.Internal, "%s", p)
			svc.logger.Errorw("panicked while calling stream server method for module request", "error", errors.WithStack(err))
			return err
		}))))

	opManager := svc.r.OperationManager()
	unaryInterceptors = append(unaryInterceptors,
		opManager.UnaryServerInterceptor, logging.UnaryServerInterceptor)
	streamInterceptors = append(streamInterceptors, opManager.StreamServerInterceptor)

	// TODO(PRODUCT-343): Add session manager interceptors

	// MaxRecvMsgSize and MaxSendMsgSize by default are 4 MB & MaxInt32 (2.1 GB)
	opts := []googlegrpc.ServerOption{
		googlegrpc.MaxRecvMsgSize(rpc.MaxMessageSize),
		googlegrpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)),
		googlegrpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
		googlegrpc.UnknownServiceHandler(svc.foreignServiceHandler),
		googlegrpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             rpc.KeepAliveTime / 2, // keep this in sync with goutils' rpc/dialer & server.
			PermitWithoutStream: true,
		}),
		googlegrpc.StatsHandler(&ocgrpc.ServerHandler{}),
	}
	server := module.NewServer(opts...)
	if tcpMode {
		svc.tcpModServer = server
	} else {
		svc.unixModServer = server
	}
	if err := server.RegisterServiceServer(ctx, &pb.RobotService_ServiceDesc, grpcserver.New(svc.r)); err != nil {
		return err
	}

	if err := svc.initStreamServer(ctx, server); err != nil {
		return err
	}

	if err := svc.initAPIResourceCollections(ctx, server); err != nil {
		return err
	}

	svc.modWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.modWorkers.Done()
		svc.logger.Debugw("module server listening", "socket path", lis.Addr())
		defer func() {
			// tcpMode starts listening on a port, not a socket file, so no need to remove.
			if !tcpMode {
				err := os.RemoveAll(filepath.Dir(addr))
				if err != nil {
					svc.logger.Debugf("RemoveAll failed: %v", err)
				}
			}
		}()
		if err := server.Serve(lis); err != nil {
			svc.logger.Errorw("failed to serve module service", "error", err)
		}
	})
	return nil
}

// StartModule starts the grpc module server.
func (svc *webService) StartModule(ctx context.Context) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if err := svc.startProtocolModuleParentServer(ctx, false); err != nil {
		return err
	}
	return svc.startProtocolModuleParentServer(ctx, true)
}

// Stop stops the main web service prior to actually closing (it leaves the module server running.)
func (svc *webService) Stop() {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.stopWeb()
}

func (svc *webService) stopWeb() {
	if svc.cancelFunc != nil {
		svc.cancelFunc()
	}
	svc.closeStreamServer()
	svc.isRunning = false
	svc.webWorkers.Wait()
}

// Close closes a webService via calls to its Cancel func.
func (svc *webService) Close(ctx context.Context) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.stopWeb()
	var errs []error
	for _, srv := range []rpc.Server{svc.tcpModServer, svc.unixModServer} {
		if srv != nil {
			errs = append(errs, srv.Stop())
		}
	}
	if svc.streamServer != nil {
		utils.UncheckedError(svc.streamServer.Close())
	}
	svc.modWorkers.Wait()
	return multierr.Combine(errs...)
}

// runWeb takes the given robot and options and runs the web server. This function will
// block until the context is done.
func (svc *webService) runWeb(ctx context.Context, options weboptions.Options) (err error) {
	if options.Network.BindAddress != "" && options.Network.Listener != nil {
		return errors.New("may only set one of network bind address or listener")
	}
	listener := options.Network.Listener

	if listener == nil {
		listener, err = net.Listen("tcp", options.Network.BindAddress)
		if err != nil {
			return err
		}
	}

	listenerTCPAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return errors.Errorf("expected *net.TCPAddr but got %T", listener.Addr())
	}

	if options.NoTLS {
		svc.logger.Warn("disabling TLS for web server")
		options.Secure = false
	} else {
		options.Secure = options.Network.TLSConfig != nil || options.Network.TLSCertFile != ""
	}
	if options.SignalingAddress == "" && !options.Secure {
		options.SignalingDialOpts = append(options.SignalingDialOpts, rpc.WithInsecure())
	}

	svc.addr = listenerTCPAddr.String()
	if options.FQDN == "" {
		options.FQDN, err = rpc.InstanceNameFromAddress(svc.addr)
		if err != nil {
			return err
		}
	}

	rpcOpts, err := svc.initRPCOptions(listenerTCPAddr, options)
	if err != nil {
		return err
	}

	ioLogger := svc.logger.Sublogger("networking")
	svc.rpcServer, err = rpc.NewServer(ioLogger, rpcOpts...)
	if err != nil {
		return err
	}

	if options.SignalingAddress == "" {
		options.SignalingAddress = svc.addr
	}

	if err := svc.rpcServer.RegisterServiceServer(
		ctx,
		&pb.RobotService_ServiceDesc,
		grpcserver.New(svc.r),
		pb.RegisterRobotServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	if err := svc.initAPIResourceCollections(ctx, svc.rpcServer); err != nil {
		return err
	}

	if err := svc.initStreamServer(ctx, svc.rpcServer); err != nil {
		return err
	}

	if options.Debug {
		if err := svc.rpcServer.RegisterServiceServer(
			ctx,
			&echopb.EchoService_ServiceDesc,
			&echoserver.Server{},
			echopb.RegisterEchoServiceHandlerFromEndpoint,
		); err != nil {
			return err
		}
	}

	httpServer, err := svc.initHTTPServer(listenerTCPAddr, options)
	if err != nil {
		return err
	}

	// Serve

	svc.webWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.webWorkers.Done()
		<-ctx.Done()
		defer func() {
			if err := httpServer.Shutdown(context.Background()); err != nil {
				svc.logger.Errorw("error shutting down", "error", err)
			}
		}()
		defer func() {
			if err := svc.rpcServer.Stop(); err != nil {
				svc.logger.Errorw("error stopping rpc server", "error", err)
			}
		}()
	})
	svc.webWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.webWorkers.Done()
		if err := svc.rpcServer.Start(); err != nil {
			svc.logger.Errorw("error starting rpc server", "error", err)
		}
	})

	var scheme string
	if options.Secure {
		scheme = "https"
	} else {
		scheme = "http"
	}
	if strings.HasPrefix(svc.addr, "[::]") {
		svc.addr = fmt.Sprintf("0.0.0.0:%d", listenerTCPAddr.Port)
	}
	listenerURL := fmt.Sprintf("%s://%s", scheme, svc.addr)
	var urlFields []interface{}
	if options.LocalFQDN == "" {
		urlFields = append(urlFields, "url", listenerURL)
	} else {
		localURL := fmt.Sprintf("%s://%s:%d", scheme, options.LocalFQDN, listenerTCPAddr.Port)
		urlFields = append(urlFields, "url", localURL, "alt_url", listenerURL)
	}
	svc.logger.Infow("serving", urlFields...)

	svc.webWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.webWorkers.Done()
		var serveErr error
		if options.Secure {
			serveErr = httpServer.ServeTLS(listener, options.Network.TLSCertFile, options.Network.TLSKeyFile)
		} else {
			serveErr = httpServer.Serve(listener)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			svc.logger.Errorw("error serving http", "error", serveErr)
		}
	})

	return err
}

// RequestCounter returns the request counter object.
func (svc *webService) RequestCounter() *RequestCounter {
	return &svc.requestCounter
}

// ModPeerConnTracker returns the ModPeerConnTracker object.
func (svc *webService) ModPeerConnTracker() *grpc.ModPeerConnTracker {
	return svc.modPeerConnTracker
}

// Initialize RPC Server options.
func (svc *webService) initRPCOptions(listenerTCPAddr *net.TCPAddr, options weboptions.Options) ([]rpc.ServerOption, error) {
	hosts := options.GetHosts(listenerTCPAddr)

	webrtcOptions := rpc.WebRTCServerOptions{
		Enable:                    true,
		EnableInternalSignaling:   true,
		ExternalSignalingDialOpts: options.SignalingDialOpts,
		ExternalSignalingAddress:  options.SignalingAddress,
		ExternalSignalingHosts:    hosts.External,
		InternalSignalingHosts:    hosts.Internal,
		Config:                    &grpc.DefaultWebRTCConfiguration,
	}
	if options.DisallowWebRTC {
		webrtcOptions = rpc.WebRTCServerOptions{
			Enable: false,
		}
	}

	rpcOpts := []rpc.ServerOption{
		rpc.WithAuthIssuer(options.FQDN),
		rpc.WithAuthAudience(options.FQDN),
		rpc.WithInstanceNames(hosts.Names...),
		rpc.WithWebRTCServerOptions(webrtcOptions),
	}
	if options.DisableMulticastDNS {
		rpcOpts = append(rpcOpts, rpc.WithDisableMulticastDNS())
	}

	var (
		unaryInterceptors  []googlegrpc.UnaryServerInterceptor
		streamInterceptors []googlegrpc.StreamServerInterceptor
	)
	unaryInterceptors = append(unaryInterceptors, grpc.EnsureTimeoutUnaryServerInterceptor)

	unaryInterceptors = append(unaryInterceptors, svc.requestCounter.UnaryInterceptor)
	streamInterceptors = append(streamInterceptors, svc.requestCounter.StreamInterceptor)

	if options.Debug {
		rpcOpts = append(rpcOpts, rpc.WithDebug())
		unaryInterceptors = append(unaryInterceptors, func(
			ctx context.Context,
			req interface{},
			info *googlegrpc.UnaryServerInfo,
			handler googlegrpc.UnaryHandler,
		) (interface{}, error) {
			ctx, span := trace.StartSpan(ctx, fmt.Sprintf("%v", req))
			defer span.End()

			return handler(ctx, req)
		})
	}

	if options.Network.TLSConfig != nil {
		rpcOpts = append(rpcOpts, rpc.WithInternalTLSConfig(options.Network.TLSConfig))
	}

	authOpts, err := svc.initAuthHandlers(listenerTCPAddr, options)
	if err != nil {
		return nil, err
	}
	rpcOpts = append(rpcOpts, authOpts...)

	opManager := svc.r.OperationManager()
	sessManagerInts := svc.r.SessionManager().ServerInterceptors()
	if sessManagerInts.UnaryServerInterceptor != nil {
		unaryInterceptors = append(unaryInterceptors, sessManagerInts.UnaryServerInterceptor)
	}
	unaryInterceptors = append(unaryInterceptors,
		opManager.UnaryServerInterceptor, logging.UnaryServerInterceptor)

	if sessManagerInts.StreamServerInterceptor != nil {
		streamInterceptors = append(streamInterceptors, sessManagerInts.StreamServerInterceptor)
	}
	streamInterceptors = append(streamInterceptors, opManager.StreamServerInterceptor)

	rpcOpts = append(
		rpcOpts,
		rpc.WithUnknownServiceHandler(svc.foreignServiceHandler),
	)

	unaryInterceptor := grpc_middleware.ChainUnaryServer(unaryInterceptors...)
	streamInterceptor := grpc_middleware.ChainStreamServer(streamInterceptors...)
	rpcOpts = append(rpcOpts,
		rpc.WithUnaryServerInterceptor(unaryInterceptor),
		rpc.WithStreamServerInterceptor(streamInterceptor),
	)

	return rpcOpts, nil
}

// Initialize authentication handler options.
func (svc *webService) initAuthHandlers(listenerTCPAddr *net.TCPAddr, options weboptions.Options) ([]rpc.ServerOption, error) {
	rpcOpts := []rpc.ServerOption{}

	if options.Managed && len(options.Auth.Handlers) == 1 {
		if options.BakedAuthEntity == "" || options.BakedAuthCreds.Type == "" {
			return nil, errors.New("expected baked in local UI credentials since managed")
		}
	}

	if len(options.Auth.Handlers) == 0 {
		rpcOpts = append(rpcOpts, rpc.WithUnauthenticated())
	} else {
		listenerAddr := listenerTCPAddr.String()
		hosts := options.GetHosts(listenerTCPAddr)
		authEntities := make([]string, len(hosts.Internal))
		copy(authEntities, hosts.Internal)
		if !options.Managed {
			// allow authentication for non-unique entities.
			// This eases direct connections via address.
			addIfNotFound := func(toAdd string) []string {
				for _, ent := range authEntities {
					if ent == toAdd {
						return authEntities
					}
				}
				return append(authEntities, toAdd)
			}
			if options.FQDN != listenerAddr {
				authEntities = addIfNotFound(listenerAddr)
			}
			if listenerTCPAddr.IP.IsLoopback() {
				// plus localhost alias
				authEntities = addIfNotFound(weboptions.LocalHostWithPort(listenerTCPAddr))
			}
		}
		if options.Secure && len(options.Auth.TLSAuthEntities) != 0 {
			rpcOpts = append(rpcOpts, rpc.WithTLSAuthHandler(options.Auth.TLSAuthEntities))
		}
		for _, handler := range options.Auth.Handlers {
			switch handler.Type {
			case rpc.CredentialsTypeAPIKey:
				apiKeys := config.ParseAPIKeys(handler)

				if len(apiKeys) == 0 {
					return nil, errors.Errorf("%q handler requires non-empty API keys", handler.Type)
				}

				rpcOpts = append(rpcOpts, rpc.WithAuthHandler(handler.Type, rpc.MakeSimpleMultiAuthPairHandler(apiKeys)))
			case rutils.CredentialsTypeRobotLocationSecret:
				locationSecrets := handler.Config.StringSlice("secrets")
				if len(locationSecrets) == 0 {
					secret := handler.Config.String("secret")
					if secret == "" {
						return nil, errors.Errorf("%q handler requires non-empty secret", handler.Type)
					}
					locationSecrets = []string{secret}
				}

				rpcOpts = append(rpcOpts, rpc.WithAuthHandler(
					handler.Type,
					rpc.MakeSimpleMultiAuthHandler(authEntities, locationSecrets),
				))
			case rpc.CredentialsTypeExternal:
			default:
				return nil, errors.Errorf("do not know how to handle auth for %q", handler.Type)
			}
		}
	}

	if options.Auth.ExternalAuthConfig != nil {
		rpcOpts = append(rpcOpts, rpc.WithExternalAuthJWKSetTokenVerifier(
			options.Auth.ExternalAuthConfig.ValidatedKeySet,
		))
	}

	return rpcOpts, nil
}

// Register every API resource grpc service here.
func (svc *webService) initAPIResourceCollections(ctx context.Context, server rpc.Server) error {
	// TODO (RSDK-144): only register necessary services
	apiRegs := resource.RegisteredAPIs()
	for api, apiReg := range apiRegs {
		apiGetter := resourceGetterForAPI{api, svc.r}
		if err := apiReg.RegisterRPCService(ctx, server, apiGetter); err != nil {
			return err
		}
	}
	return nil
}

// Initialize HTTP server.
func (svc *webService) initHTTPServer(listenerTCPAddr *net.TCPAddr, options weboptions.Options) (*http.Server, error) {
	mux := svc.initMux(options)

	httpServer, err := utils.NewPossiblySecureHTTPServer(mux, utils.HTTPServerOptions{
		Secure:         options.Secure,
		MaxHeaderBytes: rpc.MaxMessageSize,
		Addr:           listenerTCPAddr.String(),
	})
	if err != nil {
		return httpServer, err
	}
	httpServer.TLSConfig = options.Network.TLSConfig.Clone()

	return httpServer, nil
}

// Initialize multiplexer between http handlers.
func (svc *webService) initMux(options weboptions.Options) *goji.Mux {
	mux := goji.NewMux()
	// Note: used by viam-agent for health checks
	mux.HandleFunc(pat.New("/"), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("healthy")); err != nil {
			svc.logger.Warnf("unable to write healthy response: %w", err)
		}
	})

	if options.Pprof {
		mux.HandleFunc(pat.New("/debug/pprof/"), pprof.Index)
		mux.HandleFunc(pat.New("/debug/pprof/cmdline"), pprof.Cmdline)
		mux.HandleFunc(pat.New("/debug/pprof/profile"), pprof.Profile)
		mux.HandleFunc(pat.New("/debug/pprof/symbol"), pprof.Symbol)
		mux.HandleFunc(pat.New("/debug/pprof/trace"), pprof.Trace)
	}

	// serve resource graph visualization
	// TODO: hide behind option
	// TODO: accept params to display different formats
	mux.HandleFunc(pat.New("/debug/graph"), svc.handleVisualizeResourceGraph)

	// serve restart status
	mux.HandleFunc(pat.New("/restart_status"), svc.handleRestartStatus)

	prefix := "/viam"
	addPrefix := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := prefix + r.URL.Path
			rp := prefix + r.URL.RawPath
			if len(p) > len(r.URL.Path) && (r.URL.RawPath == "" || len(rp) > len(r.URL.RawPath)) {
				r2 := new(http.Request)
				*r2 = *r
				r2.URL = new(url.URL)
				*r2.URL = *r.URL
				r2.URL.Path = p
				r2.URL.RawPath = rp
				h.ServeHTTP(w, r2)
			} else {
				http.NotFound(w, r)
			}
		})
	}

	// for urls with /api, add /viam to the path so that it matches with the paths defined in protobuf.
	corsHandler := cors.AllowAll()
	mux.Handle(pat.New("/api/*"), corsHandler.Handler(addPrefix(svc.rpcServer.GatewayHandler())))
	mux.Handle(pat.New("/*"), corsHandler.Handler(svc.rpcServer.GRPCHandler()))

	return mux
}

// foreignServiceHandler is a bidi-streaming RPC service handler to support custom APIs.
// It is invoked instead of returning the "unimplemented" gRPC error whenever a request is received for
// an unregistered service or method. These method could be registered on a remote viam-server or a module server
// so this handler will attempt to route the request to the correct next node in the chain.
func (svc *webService) foreignServiceHandler(srv interface{}, stream googlegrpc.ServerStream) error {
	// method will be in the form of PackageName.ServiceName/MethodName
	method, ok := googlegrpc.MethodFromServerStream(stream)
	if !ok {
		return grpc.UnimplementedError
	}
	subType, methodDesc, err := robot.TypeAndMethodDescFromMethod(svc.r, method)
	if err != nil {
		return err
	}

	firstMsg := dynamic.NewMessage(methodDesc.GetInputType())

	// The stream blocks until it receives a message and attempts to deserialize
	// the message into firstMsg - it will error out if the received message cannot
	// be marshalled into the expected type.
	if err := stream.RecvMsg(firstMsg); err != nil {
		return err
	}

	// We expect each message to contain a "name" argument which will allow us to route
	// the message towards the correct destination.
	resource, fqName, err := robot.ResourceFromProtoMessage(svc.r, firstMsg, subType.API)
	if err != nil {
		svc.logger.Errorw("unable to route foreign message", "error", err)
		return err
	}

	if fqName.ContainsRemoteNames() {
		firstMsg.SetFieldByName("name", fqName.PopRemote().ShortName())
	}

	foreignRes, ok := resource.(*grpc.ForeignResource)
	if !ok {
		svc.logger.Errorf("expected resource to be a foreign RPC resource but was %T", foreignRes)
		return grpc.UnimplementedError
	}

	foreignClient := foreignRes.NewStub()

	// see https://github.com/fullstorydev/grpcurl/blob/76bbedeed0ec9b6e09ad1e1cb88fffe4726c0db2/invoke.go
	switch {
	case methodDesc.IsClientStreaming() && methodDesc.IsServerStreaming():

		ctx, cancel := context.WithCancel(stream.Context())
		defer cancel()

		bidiStream, err := foreignClient.InvokeRpcBidiStream(ctx, methodDesc)
		if err != nil {
			return err
		}

		var wg sync.WaitGroup
		var sendErr atomic.Pointer[error]

		defer wg.Wait()

		wg.Add(1)
		utils.PanicCapturingGo(func() {
			defer wg.Done()

			var err error
			// process first message before waiting for more messages
			err = bidiStream.SendMsg(firstMsg)
			for err == nil {
				msg := dynamic.NewMessage(methodDesc.GetInputType())
				if err = stream.RecvMsg(msg); err != nil {
					if errors.Is(err, io.EOF) {
						err = bidiStream.CloseSend()
						break
					}
					cancel()
					break
				}
				// remove a remote from the name if needed
				if fqName.ContainsRemoteNames() {
					msg.SetFieldByName("name", fqName.PopRemote().ShortName())
				}
				err = bidiStream.SendMsg(msg)
			}

			if err != nil {
				sendErr.Store(&err)
			}
		})

		for {
			resp, err := bidiStream.RecvMsg()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return err
				}
				break
			}

			if err := stream.SendMsg(resp); err != nil {
				cancel()
				return err
			}
		}

		wg.Wait()
		if err := sendErr.Load(); err != nil && !errors.Is(*err, io.EOF) {
			return *err
		}

		return nil
	case methodDesc.IsClientStreaming():
		clientStream, err := foreignClient.InvokeRpcClientStream(stream.Context(), methodDesc)
		if err != nil {
			return err
		}
		// process first message before waiting for more messages
		err = clientStream.SendMsg(firstMsg)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		for err == nil {
			msg := dynamic.NewMessage(methodDesc.GetInputType())
			if err := stream.RecvMsg(msg); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return err
			}
			if fqName.ContainsRemoteNames() {
				msg.SetFieldByName("name", fqName.PopRemote().ShortName())
			}
			if err := clientStream.SendMsg(msg); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
		}
		resp, err := clientStream.CloseAndReceive()
		if err != nil {
			return err
		}
		return stream.SendMsg(resp)
	case methodDesc.IsServerStreaming():
		secondMsg := dynamic.NewMessage(methodDesc.GetInputType())
		if err := stream.RecvMsg(secondMsg); err == nil {
			return errors.Errorf(
				"method %q is a server-streaming RPC, but request data contained more than 1 message",
				methodDesc.GetFullyQualifiedName())
		} else if !errors.Is(err, io.EOF) {
			return err
		}

		serverStream, err := foreignClient.InvokeRpcServerStream(stream.Context(), methodDesc, firstMsg)
		if err != nil {
			return err
		}

		for {
			resp, err := serverStream.RecvMsg()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return err
				}
				break
			}
			if err := stream.SendMsg(resp); err != nil {
				return err
			}
		}

		return nil
	default:
		invokeResp, err := foreignClient.InvokeRpc(stream.Context(), methodDesc, firstMsg)
		if err != nil {
			return err
		}
		return stream.SendMsg(invokeResp)
	}
}

type stats struct {
	RPCServer any
}

// Stats returns ftdc data on behalf of the rpcServer and other web services.
func (svc *webService) Stats() any {
	// RSDK-9369: It's not ideal to block in `Stats`. But we don't today expect this to be
	// problematic, and alternatives are more complex/expensive.
	svc.mu.Lock()
	defer svc.mu.Unlock()

	return stats{svc.rpcServer.Stats()}
}

// RestartStatusResponse is the JSON response of the `restart_status` HTTP
// endpoint.
type RestartStatusResponse struct {
	// RestartAllowed represents whether this instance of the viamserver can be
	// safely restarted.
	RestartAllowed bool `json:"restart_allowed"`
	// DoesNotHandleNeedsRestart represents whether this instance of the viamserver does
	// not check for the need to restart against app itself and, thus, needs agent to do so.
	// Newer versions of viamserver (>= v0.9x.0) will report true for this value, while
	// older versions won't report it at all, and agent should let viamserver handle
	// NeedsRestart logic.
	DoesNotHandleNeedsRestart bool `json:"does_not_handle_needs_restart,omitempty"`
}

// Handles the `/restart_status` endpoint.
func (svc *webService) handleRestartStatus(w http.ResponseWriter, r *http.Request) {
	response := RestartStatusResponse{
		RestartAllowed:            svc.r.RestartAllowed(),
		DoesNotHandleNeedsRestart: true,
	}

	w.Header().Set("Content-Type", "application/json")
	// Only log errors from encoding here. A failure to encode should never
	// happen.
	utils.UncheckedError(json.NewEncoder(w).Encode(response))
}
