package rpc

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"go.uber.org/multierr"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

// A Server provides a convenient way to get a gRPC server up and running
// with HTTP facilities.
type Server interface {
	Start() error
	Serve(listener net.Listener) (err error)
	Stop() error
	RegisterServiceServer(
		ctx context.Context,
		svcDesc *grpc.ServiceDesc,
		svcServer interface{},
		svcHandlers ...RegisterServiceHandlerFromEndpointFunc,
	) error
	GatewayHandler() http.Handler
	GRPCHandler() http.Handler
	http.Handler
}

type simpleServer struct {
	mu                 sync.Mutex
	grpcListener       net.Listener
	grpcServer         *grpc.Server
	grpcWebServer      *grpcweb.WrappedGrpcServer
	grpcGatewayHandler *runtime.ServeMux
	httpServer         *http.Server
	secure             bool
}

func NewServerWithListener(grpcListener net.Listener) Server {
	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)
	grpcWebServer := grpcweb.WrapServer(grpcServer)
	grpcGatewayHandler := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames: true,
			},
		}),
	)

	httpServer := &http.Server{
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return &simpleServer{
		grpcListener:       grpcListener,
		grpcServer:         grpcServer,
		grpcWebServer:      grpcWebServer,
		grpcGatewayHandler: grpcGatewayHandler,
		httpServer:         httpServer,
	}
}

func NewServer() (Server, error) {
	grpcListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	return NewServerWithListener(grpcListener), nil
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
	} else if ss.grpcWebServer.IsGrpcWebRequest(r) {
		return requestTypeGRPCWeb
	}
	return requestTypeNone
}

// GatewayHandler returns a handler for gateway based gRPC requests.
// See: https://github.com/grpc-ecosystem/grpc-gateway
func (ss *simpleServer) GatewayHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ss.grpcGatewayHandler.ServeHTTP(w, r)
	})
}

// GRPCHandler returns a handler for standard grpc/grpc-web requests which
// expect to be served from a root path.
func (ss *simpleServer) GRPCHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	switch ss.getRequestType(r) {
	case requestTypeGRPC:
		ss.grpcServer.ServeHTTP(w, r)
	case requestTypeGRPCWeb:
		ss.grpcWebServer.ServeHTTP(w, r)
	default:
		ss.grpcGatewayHandler.ServeHTTP(w, r)
	}
}

func (ss *simpleServer) Start() error {
	return ss.grpcServer.Serve(ss.grpcListener)
}

func (ss *simpleServer) Serve(listener net.Listener) (err error) {
	var handler http.Handler = ss
	if !ss.secure {
		h2s := &http2.Server{}
		ss.httpServer.Addr = listener.Addr().String()
		handler = h2c.NewHandler(ss, h2s)
	}
	ss.httpServer.Addr = listener.Addr().String()
	ss.httpServer.Handler = handler
	var errMu sync.Mutex
	go func() {
		if serveErr := ss.httpServer.Serve(listener); serveErr != http.ErrServerClosed {
			errMu.Lock()
			err = multierr.Combine(err, serveErr)
			errMu.Unlock()
		}
	}()
	serveErr := ss.Start()
	errMu.Lock()
	err = multierr.Combine(err, serveErr)
	errMu.Unlock()
	return
}

func (ss *simpleServer) Stop() error {
	defer ss.grpcServer.Stop()
	return ss.httpServer.Shutdown(context.Background())
}

type RegisterServiceHandlerFromEndpointFunc func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error)

func (ss *simpleServer) RegisterServiceServer(
	ctx context.Context,
	svcDesc *grpc.ServiceDesc,
	svcServer interface{},
	svcHandlers ...RegisterServiceHandlerFromEndpointFunc,
) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.grpcServer.RegisterService(svcDesc, svcServer)
	if len(svcHandlers) != 0 {
		addr := ss.grpcListener.Addr().String()
		var opts []grpc.DialOption
		if !ss.secure {
			opts = append(opts, grpc.WithInsecure())
		}
		for _, h := range svcHandlers {
			if err := h(ctx, ss.grpcGatewayHandler, addr, opts); err != nil {
				return err
			}
		}
	}
	return nil
}
