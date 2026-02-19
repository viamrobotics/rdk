package module

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.viam.com/rdk/module/modutil"
)

// NewServer returns a new (module specific) rpc.Server.
func NewServer(opts ...grpc.ServerOption) rpc.Server {
	// Some modules depend on being able to propagate trace context even if
	// viam-server isn't recording anything. Set up propagation here to avoid
	// breaking that use case.
	traceProvider := trace.GetProvider()
	otelHandler := otelgrpc.NewServerHandler(
		otelgrpc.WithTracerProvider(traceProvider),
		otelgrpc.WithPropagators(propagation.TraceContext{}),
	)
	grpcHandler := grpc.StatsHandler(otelHandler)
	opts = append(opts, grpcHandler)

	s := &Server{server: grpc.NewServer(opts...)}
	reflection.Register(s.server)
	return s
}

// Server provides an rpc.Server wrapper around a grpc.Server.
type Server struct {
	mu     sync.RWMutex
	server *grpc.Server
	addr   net.Addr
}

// InternalAddr returns the internal address of the server.
func (s *Server) InternalAddr() net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addr
}

// InstanceNames is unsupported.
func (s *Server) InstanceNames() []string {
	return []string{}
}

// EnsureAuthed is unsupported.
func (s *Server) EnsureAuthed(ctx context.Context) (context.Context, error) {
	return nil, errors.New("EnsureAuthed is unsupported")
}

// Start is unsupported.
func (s *Server) Start() error {
	return errors.New("start unsupported on module grpc server")
}

// Serve begins listening/serving grpc.
func (s *Server) Serve(listener net.Listener) error {
	s.mu.Lock()
	s.addr = listener.Addr()
	s.mu.Unlock()
	return s.server.Serve(listener)
}

// ServeTLS is unsupported.
func (s *Server) ServeTLS(listener net.Listener, certFile, keyFile string, tlsConfig *tls.Config) error {
	return errors.New("tls unsupported on module grpc server")
}

// Stop performs a GracefulStop() on the underlying grpc server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addr = &net.UnixAddr{}
	s.server.GracefulStop()
	return nil
}

// RegisterServiceServer associates a service description with
// its implementation along with any gateway handlers.
func (s *Server) RegisterServiceServer(
	ctx context.Context,
	svcDesc *grpc.ServiceDesc,
	svcServer interface{},
	svcHandlers ...rpc.RegisterServiceHandlerFromEndpointFunc,
) error {
	s.server.RegisterService(svcDesc, svcServer)
	return nil
}

// GatewayHandler is unsupported.
func (s *Server) GatewayHandler() http.Handler {
	return &httpHandler{}
}

// GRPCHandler is unsupported.
func (s *Server) GRPCHandler() http.Handler {
	return &httpHandler{}
}

// ServeHTTP is unsupported.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := &httpHandler{}
	h.ServeHTTP(w, r)
}

// Stats is unsupported.
func (s *Server) Stats() any {
	return nil
}

type httpHandler struct{}

// ServeHTTP returns only an error message.
func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "http unsupported", http.StatusInternalServerError)
}

// HandlerMap is the format for api->model pairs that the module will service.
// Ex: mymap["rdk:component:motor"] = ["acme:marine:thruster", "acme:marine:outboard"].
type HandlerMap = modutil.HandlerMap

// NewHandlerMapFromProto converts protobuf to HandlerMap.
func NewHandlerMapFromProto(ctx context.Context, pMap *pb.HandlerMap, conn rpc.ClientConn) (HandlerMap, error) {
	return modutil.NewHandlerMapFromProto(ctx, pMap, conn)
}
