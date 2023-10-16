package module

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// NewServer returns a new (module specific) rpc.Server.
func NewServer(unary []grpc.UnaryServerInterceptor, stream []grpc.StreamServerInterceptor) rpc.Server {
	s := &Server{server: grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unary...)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(stream...)),
	)}
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

type httpHandler struct{}

// ServeHTTP returns only an error message.
func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "http unsupported", http.StatusInternalServerError)
}
