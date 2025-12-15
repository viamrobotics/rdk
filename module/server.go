package module

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/multierr"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
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
type HandlerMap map[resource.RPCAPI][]resource.Model

// ToProto converts the HandlerMap to a protobuf representation.
func (h HandlerMap) ToProto() *pb.HandlerMap {
	pMap := &pb.HandlerMap{}
	for s, models := range h {
		subtype := &robotpb.ResourceRPCSubtype{
			Subtype: protoutils.ResourceNameToProto(resource.Name{
				API:  s.API,
				Name: "",
			}),
			ProtoService: s.ProtoSvcName,
		}

		handler := &pb.HandlerDefinition{Subtype: subtype}
		for _, m := range models {
			handler.Models = append(handler.Models, m.String())
		}
		pMap.Handlers = append(pMap.Handlers, handler)
	}
	return pMap
}

// NewHandlerMapFromProto converts protobuf to HandlerMap.
func NewHandlerMapFromProto(ctx context.Context, pMap *pb.HandlerMap, conn rpc.ClientConn) (HandlerMap, error) {
	hMap := make(HandlerMap)
	refClient := grpcreflect.NewClientV1Alpha(ctx, reflectpb.NewServerReflectionClient(conn))
	defer refClient.Reset()
	reflSource := grpcurl.DescriptorSourceFromServer(ctx, refClient)

	var errs error
	for _, h := range pMap.GetHandlers() {
		api := protoutils.ResourceNameFromProto(h.Subtype.Subtype).API
		rpcAPI := &resource.RPCAPI{
			API: api,
		}
		// due to how tagger is setup in the api we cannot use reflection on the discovery service currently
		// for now we will skip the reflection step for discovery until the issue is resolved.
		// TODO(RSDK-9718) - remove the skip.
		if api != discovery.API {
			symDesc, err := reflSource.FindSymbol(h.Subtype.ProtoService)
			if err != nil {
				errs = multierr.Combine(errs, err)
				if errors.Is(err, grpcurl.ErrReflectionNotSupported) {
					return nil, errs
				}
				continue
			}
			svcDesc, ok := symDesc.(*desc.ServiceDescriptor)
			if !ok {
				return nil, fmt.Errorf("expected descriptor to be service descriptor but got %T", symDesc)
			}
			rpcAPI.Desc = svcDesc
		}
		for _, m := range h.Models {
			model, err := resource.NewModelFromString(m)
			if err != nil {
				return nil, err
			}
			hMap[*rpcAPI] = append(hMap[*rpcAPI], model)
		}
	}
	return hMap, errs
}
