// Package server provides the remote procedure call (RPC) server based on gRPC.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	webrtcpb "go.viam.com/core/proto/rpc/webrtc/v1"
	"go.viam.com/core/rpc"
	rpcwebrtc "go.viam.com/core/rpc/webrtc"
	"go.viam.com/core/utils"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"go.uber.org/multierr"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// A Server provides a convenient way to get a gRPC server up and running
// with HTTP facilities.
type Server interface {
	// InternalAddr returns the address from the listener used for
	// gRPC communications. It may be the same listener the server
	// was constructed with.
	InternalAddr() net.Addr

	// Start only starts up the internal gRPC server.
	Start() error

	// Serve will externally serve, on the given listener, the
	// all in one handler described by http.Handler.
	Serve(listener net.Listener) (err error)

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
}

type simpleServer struct {
	mu                   sync.Mutex
	grpcListener         net.Listener
	grpcServer           *grpc.Server
	grpcWebServer        *grpcweb.WrappedGrpcServer
	grpcGatewayHandler   *runtime.ServeMux
	httpServer           *http.Server
	webrtcServer         *rpcwebrtc.Server
	webrtcAnswerer       *rpcwebrtc.SignalingAnswerer
	serviceServerCancels []func()
	serviceServers       []interface{}
	secure               bool
	stopped              bool
	logger               golog.Logger
}

// Options change the runtime behavior of the server.
type Options struct {
	WebRTC WebRTCOptions
}

// WebRTCOptions control how WebRTC is utilized in a server.
type WebRTCOptions struct {
	// Enable controls if WebRTC should be turned on. It is disabled
	// by default since signaling has the potential to open up random
	// ports on the host which may not be expected.
	Enable bool

	// Insecure determines if communications are expected to be insecure or not.
	Insecure bool

	// EnableSignaling controls if this server will provide SDP signaling
	// assistance.
	EnableSignaling bool

	// SignalingAddress specifies where the WebRTC signaling
	// answerer should connect to and "listen" from. If it is empty,
	// it will connect to the server's internal address acting as
	// an answerer for itself.
	SignalingAddress string

	// SignalingHost specifies what host is being listened for.
	SignalingHost string
}

// NewWithListener returns a new server ready to be started that
// will listen on the given listener.
func NewWithListener(
	grpcListener net.Listener,
	opts Options,
	logger golog.Logger,
) (Server, error) {
	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)
	grpcWebServer := grpcweb.WrapServer(grpcServer, grpcweb.WithOriginFunc(func(origin string) bool {
		// TODO(erd): limit this to some base url
		return true
	}))
	grpcGatewayHandler := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.HTTPBodyMarshaler{rpc.JSONPB}))

	httpServer := &http.Server{
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 24,
	}

	server := &simpleServer{
		grpcListener:       grpcListener,
		grpcServer:         grpcServer,
		grpcWebServer:      grpcWebServer,
		grpcGatewayHandler: grpcGatewayHandler,
		httpServer:         httpServer,
		logger:             logger,
	}

	if opts.WebRTC.EnableSignaling || (opts.WebRTC.Enable && opts.WebRTC.SignalingAddress == "") {
		logger.Info("will run local signaling service")
		if err := server.RegisterServiceServer(
			context.Background(),
			&webrtcpb.SignalingService_ServiceDesc,
			rpcwebrtc.NewSignalingServer(),
			webrtcpb.RegisterSignalingServiceHandlerFromEndpoint,
		); err != nil {
			return nil, err
		}
	}

	if opts.WebRTC.Enable {
		server.webrtcServer = rpcwebrtc.NewServer(logger)
		address := opts.WebRTC.SignalingAddress
		if address == "" {
			address = grpcListener.Addr().String()
		}
		signalingHost := opts.WebRTC.SignalingHost
		if signalingHost == "" {
			signalingHost = "local"
		}
		logger.Infow("will run signaling answerer", "address", address, "host", signalingHost)
		server.webrtcAnswerer = rpcwebrtc.NewSignalingAnswerer(
			address,
			signalingHost,
			server.webrtcServer,
			opts.WebRTC.Insecure,
			logger,
		)
	}

	return server, nil
}

// NewWithOptions returns a new server ready to be started that
// will listen on some random port bound to localhost.
func NewWithOptions(opts Options, logger golog.Logger) (Server, error) {
	grpcListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	return NewWithListener(grpcListener, opts, logger)
}

// New returns a new server ready to be started that
// will listen on some random port bound to localhost.
func New(logger golog.Logger) (Server, error) {
	return NewWithOptions(Options{}, logger)
}

type requestType int

const (
	requestTypeNone requestType = iota
	requestTypeGRPC
	requestTypeGRPCWeb
)

func (ss *simpleServer) getRequestType(r *http.Request) requestType {
	if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
		return requestTypeGRPC
	} else if ss.grpcWebServer.IsAcceptableGrpcCorsRequest(r) || ss.grpcWebServer.IsGrpcWebRequest(r) {
		return requestTypeGRPCWeb
	}
	return requestTypeNone
}

func requestWithHost(r *http.Request) *http.Request {
	if r.Host == "" {
		return r
	}
	host := strings.Split(r.Host, ":")[0]
	return r.WithContext(rpc.ContextWithHost(r.Context(), host))
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
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
}

// ServeHTTP is an all-in-one handler for any kind of gRPC traffic. This is useful
// in a scenario where all gRPC is served from the root path due to limitations of normal
// gRPC being served from a non-root path.
func (ss *simpleServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = requestWithHost(r)
	switch ss.getRequestType(r) {
	case requestTypeGRPC:
		ss.grpcServer.ServeHTTP(w, r)
	case requestTypeGRPCWeb:
		ss.grpcWebServer.ServeHTTP(w, r)
	default:
		ss.grpcGatewayHandler.ServeHTTP(w, r)
	}
}

func (ss *simpleServer) InternalAddr() net.Addr {
	return ss.grpcListener.Addr()
}

func (ss *simpleServer) Start() error {
	var err error
	var errMu sync.Mutex
	utils.PanicCapturingGo(func() {
		if serveErr := ss.grpcServer.Serve(ss.grpcListener); serveErr != nil {
			errMu.Lock()
			err = multierr.Combine(err, serveErr)
			errMu.Unlock()
		}
	})

	if ss.webrtcAnswerer == nil {
		return nil
	}

	errMu.Lock()
	if startErr := ss.webrtcAnswerer.Start(); startErr != nil && utils.FilterOutError(startErr, context.Canceled) != nil {
		err = multierr.Combine(err, fmt.Errorf("error starting WebRTC answerer: %w", startErr))
	}
	capErr := err
	errMu.Unlock()

	if capErr != nil {
		ss.grpcServer.Stop()
	}

	errMu.Lock()
	defer errMu.Unlock()
	return err
}

func (ss *simpleServer) Serve(listener net.Listener) error {
	var handler http.Handler = ss
	if !ss.secure {
		h2s := &http2.Server{}
		ss.httpServer.Addr = listener.Addr().String()
		handler = h2c.NewHandler(ss, h2s)
	}
	ss.httpServer.Addr = listener.Addr().String()
	ss.httpServer.Handler = handler
	var err error
	var errMu sync.Mutex
	utils.ManagedGo(func() {
		if serveErr := ss.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errMu.Lock()
			err = multierr.Combine(err, serveErr)
			errMu.Unlock()
		}
	}, nil)
	startErr := ss.Start()
	errMu.Lock()
	err = multierr.Combine(err, startErr)
	errMu.Unlock()
	return err
}

func (ss *simpleServer) Stop() (err error) {
	ss.mu.Lock()
	if ss.stopped {
		ss.mu.Unlock()
		return nil
	}
	ss.stopped = true
	ss.mu.Unlock()
	ss.logger.Info("stopping server")
	defer ss.grpcServer.Stop()
	ss.logger.Info("canceling service servers for gateway")
	for _, cancel := range ss.serviceServerCancels {
		cancel()
	}
	ss.logger.Info("service servers for gateway canceled")
	ss.logger.Info("closing service servers")
	for _, srv := range ss.serviceServers {
		err = multierr.Combine(err, utils.TryClose(srv))
	}
	ss.logger.Info("service servers closed")
	if ss.webrtcAnswerer != nil {
		ss.logger.Info("stopping WebRTC answerer")
		ss.webrtcAnswerer.Stop()
		ss.logger.Info("WebRTC answerer stopped")
	}
	if ss.webrtcServer != nil {
		ss.logger.Info("stopping WebRTC server")
		ss.webrtcServer.Stop()
		ss.logger.Info("WebRTC server stopped")
	}
	ss.logger.Info("shutting down HTTP server")
	err = multierr.Combine(err, ss.httpServer.Shutdown(context.Background()))
	ss.logger.Info("HTTP server shut down")
	ss.logger.Info("stopped cleanly")
	return nil
}

// A RegisterServiceHandlerFromEndpointFunc is a means to have a service attach itself to a gRPC gateway mux.
type RegisterServiceHandlerFromEndpointFunc func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error)

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
	ss.serviceServers = append(ss.serviceServers, svcServer)
	ss.grpcServer.RegisterService(svcDesc, svcServer)
	if ss.webrtcServer != nil {
		ss.webrtcServer.RegisterService(svcDesc, svcServer)
	}
	if len(svcHandlers) != 0 {
		addr := ss.grpcListener.Addr().String()
		opts := []grpc.DialOption{grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1 << 24))}
		if !ss.secure {
			opts = append(opts, grpc.WithInsecure())
		}
		for _, h := range svcHandlers {
			if err := h(stopCtx, ss.grpcGatewayHandler, addr, opts); err != nil {
				return err
			}
		}
	}
	return nil
}
