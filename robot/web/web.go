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
	"strings"
	"sync"
	"sync/atomic"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	echopb "go.viam.com/utils/proto/rpc/examples/echo/v1"
	"go.viam.com/utils/rpc"
	echoserver "go.viam.com/utils/rpc/examples/echo/server"
	"goji.io"
	"goji.io/pat"
	googlegrpc "google.golang.org/grpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	grpcserver "go.viam.com/rdk/robot/server"
	weboptions "go.viam.com/rdk/robot/web/options"
	rutils "go.viam.com/rdk/utils"
)

// SubtypeName is a constant that identifies the internal web resource subtype string.
const SubtypeName = "web"

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
	ModuleAddress() string

	Stats() any
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
func (svc *webService) ModuleAddress() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return svc.modAddr
}

// StartModule starts the grpc module server.
func (svc *webService) StartModule(ctx context.Context) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.modServer != nil {
		return errors.New("module service already started")
	}

	var lis net.Listener
	var addr string
	if err := module.MakeSelfOwnedFilesFunc(func() error {
		dir, err := rutils.PlatformMkdirTemp("", "viam-module-*")
		if err != nil {
			return errors.WithMessage(err, "module startup failed")
		}

		if rutils.ViamTCPSockets() {
			addr = "127.0.0.1:14998"
			lis, err = net.Listen("tcp", addr)
		} else {
			addr, err = module.CreateSocketAddress(dir, "parent")
			if err != nil {
				return errors.WithMessage(err, "module startup failed")
			}
			lis, err = net.Listen("tcp", addr)
		}
		if err != nil {
			return errors.WithMessage(err, "failed to listen")
		}
		svc.modAddr = addr
		return nil
	}); err != nil {
		return err
	}
	var (
		unaryInterceptors  []googlegrpc.UnaryServerInterceptor
		streamInterceptors []googlegrpc.StreamServerInterceptor
	)

	unaryInterceptors = append(unaryInterceptors, grpc.EnsureTimeoutUnaryServerInterceptor)

	opManager := svc.r.OperationManager()
	unaryInterceptors = append(unaryInterceptors,
		opManager.UnaryServerInterceptor, logging.UnaryServerInterceptor)
	streamInterceptors = append(streamInterceptors, opManager.StreamServerInterceptor)
	// TODO(PRODUCT-343): Add session manager interceptors

	opts := []googlegrpc.ServerOption{
		googlegrpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)),
		googlegrpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
		googlegrpc.UnknownServiceHandler(svc.foreignServiceHandler),
	}
	svc.modServer = module.NewServer(opts...)
	if err := svc.modServer.RegisterServiceServer(ctx, &pb.RobotService_ServiceDesc, grpcserver.New(svc.r)); err != nil {
		return err
	}
	if err := svc.initAPIResourceCollections(ctx, true); err != nil {
		return err
	}
	if err := svc.refreshResources(); err != nil {
		return err
	}

	svc.modWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.modWorkers.Done()
		svc.logger.Debugw("module server listening", "socket path", lis.Addr())
		defer utils.UncheckedErrorFunc(func() error { return os.RemoveAll(filepath.Dir(addr)) })
		if err := svc.modServer.Serve(lis); err != nil {
			svc.logger.Errorw("failed to serve module service", "error", err)
		}
	})
	return nil
}

func (svc *webService) refreshResources() error {
	resources := make(map[resource.Name]resource.Resource)
	for _, name := range svc.r.ResourceNames() {
		resource, err := svc.r.ResourceByName(name)
		if err != nil {
			continue
		}
		resources[name] = resource
	}
	return svc.updateResources(resources)
}

// updateResources gets every existing resource on the robot's resource graph and updates ResourceAPICollection object
// with the correct resources, include deleting ones which have been removed from the resource graph.
func (svc *webService) updateResources(resources map[resource.Name]resource.Resource) error {
	groupedResources := make(map[resource.API]map[resource.Name]resource.Resource)
	for n, v := range resources {
		r, ok := groupedResources[n.API]
		if !ok {
			r = make(map[resource.Name]resource.Resource)
		}
		r[n] = v
		groupedResources[n.API] = r
	}

	// For a given API that the web service has resources for, we get the new set of resources we should be updated with.
	// If we find a set of resources, `coll.ReplaceAll` will do the work of adding any new resources and deleting old ones.
	//
	// If there are no input resources of the given API, we call `coll.ReplaceAll` with an empty input such that it will
	// remove any existing resources.
	for api, coll := range svc.services {
		group, ok := groupedResources[api]
		if !ok {
			// create an empty map of resources if one does not exist
			group = make(map[resource.Name]resource.Resource)
		}
		if err := coll.ReplaceAll(group); err != nil {
			return err
		}
		delete(groupedResources, api)
	}

	// If there are any groupedResources remaining, check if they are registered/internal/remote.
	//  * Custom APIs are registered and do not have a dedicated gRPC service as requests for them are routed through the
	//    foreignServiceHandler.
	//  * Internal services do not have an associated gRPC API and so can be safely ignored.
	//  * Remote resources with unregistered APIs are possibly handled by the remote robot and requests would be routed through the
	//    foreignServiceHandler.
	for api, group := range groupedResources {
		apiRegs := resource.RegisteredAPIs()
		_, ok := apiRegs[api]
		if ok {
			// If registered, the API is most likely a custom API registered through modular resources.
			continue
		}
		// Log a warning here to remind users to register their APIs.
		if api.Type.Namespace != resource.APINamespaceRDKInternal {
			for n := range group {
				if !n.ContainsRemoteNames() {
					svc.logger.Warnw(
						"missing registration for api, resources with this API will be unreachable through a client", "api", n.API)
					break
				}
			}
		}
	}
	return nil
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
	svc.isRunning = false
	svc.webWorkers.Wait()
}

// Close closes a webService via calls to its Cancel func.
func (svc *webService) Close(ctx context.Context) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.stopWeb()
	var err error
	if svc.modServer != nil {
		err = svc.modServer.Stop()
	}
	svc.modWorkers.Wait()
	return err
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

	options.Secure = options.Network.TLSConfig != nil || options.Network.TLSCertFile != ""
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

	if err := svc.initAPIResourceCollections(ctx, false); err != nil {
		return err
	}
	if err := svc.refreshResources(); err != nil {
		return err
	}

	if err := svc.initStreamServer(ctx); err != nil {
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
		svc.closeStreamServer()
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
		OnPeerAdded:               options.WebRTCOnPeerAdded,
		OnPeerRemoved:             options.WebRTCOnPeerRemoved,
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

	var unaryInterceptors []googlegrpc.UnaryServerInterceptor

	unaryInterceptors = append(unaryInterceptors, grpc.EnsureTimeoutUnaryServerInterceptor)

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

	var streamInterceptors []googlegrpc.StreamServerInterceptor

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
				apiKeys := parseAPIKeys(handler)

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

func parseAPIKeys(handler config.AuthHandlerConfig) map[string]string {
	apiKeys := map[string]string{}
	for k := range handler.Config {
		// if it is not a legacy api key indicated by "key(s)" key
		// current api keys will follow format { [keyId]: [key] }
		if k != "keys" && k != "key" {
			apiKeys[k] = handler.Config.String(k)
		}
	}
	return apiKeys
}

// Register every API resource grpc service here.
func (svc *webService) initAPIResourceCollections(ctx context.Context, mod bool) error {
	// TODO (RSDK-144): only register necessary services
	apiRegs := resource.RegisteredAPIs()
	for s, rs := range apiRegs {
		apiResColl, ok := svc.services[s]
		if !ok {
			apiResColl = rs.MakeEmptyCollection()
			svc.services[s] = apiResColl
		}

		server := svc.rpcServer
		if mod {
			server = svc.modServer
		}
		if err := rs.RegisterRPCService(ctx, server, apiResColl); err != nil {
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

// RestartStatusResponse is the JSON response of the `restart_status` HTTP
// endpoint.
type RestartStatusResponse struct {
	// RestartAllowed represents whether this instance of the viam-server can be
	// safely restarted.
	RestartAllowed bool `json:"restart_allowed"`
}

// Handles the `/restart_status` endpoint.
func (svc *webService) handleRestartStatus(w http.ResponseWriter, r *http.Request) {
	localRobot, isLocal := svc.r.(robot.LocalRobot)
	if !isLocal {
		return
	}

	response := RestartStatusResponse{RestartAllowed: localRobot.RestartAllowed()}

	w.Header().Set("Content-Type", "application/json")
	// Only log errors from encoding here. A failure to encode should never
	// happen.
	utils.UncheckedError(json.NewEncoder(w).Encode(response))
}
