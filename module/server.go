package module

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.viam.com/rdk/operation"
)

// NewServer returns a new (module specific) rpc.Server.
func NewServer(opManager *operation.Manager) rpc.Server {
	s := &Server{server: googlegrpc.NewServer(
		googlegrpc.UnaryInterceptor(opManager.UnaryServerInterceptor),
		googlegrpc.StreamInterceptor(opManager.StreamServerInterceptor),
	)}
	reflection.Register(s.server)
	return s
}

// Server provides an rpc.Server wrapper around a grpc.Server.
type Server struct {
	mu     sync.RWMutex
	server *googlegrpc.Server
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

// Start is unsupported.
func (s *Server) Start() error {
	return errors.New("start unsupported on grpc lite service")
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
	return errors.New("tls unsupoorted on grpc lite service")
}

// Stop performs a GracefulStop() on the underlying grpc service.
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
	svcDesc *googlegrpc.ServiceDesc,
	svcServer interface{},
	svcHandlers ...rpc.RegisterServiceHandlerFromEndpointFunc,
) error {
	s.server.RegisterService(svcDesc, svcServer)
	return nil
}

// GatewayHandler is unsupported.
func (s *Server) GatewayHandler() http.Handler {
	return nil
}

// GRPCHandler is unsupported.
func (s *Server) GRPCHandler() http.Handler {
	return nil
}

// ServeHTTP is unsupported.
func (s *Server) ServeHTTP(http.ResponseWriter, *http.Request) {}
