// Package summation defines a simple number summing service.
package summationapi

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	pb "go.viam.com/rdk/examples/customresources/proto/api/service/summation/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	goutils "go.viam.com/utils"
)

var ResourceSubtype = resource.NewSubtype(
	resource.Namespace("acme"),
	resource.ResourceTypeService,
	resource.SubtypeName("summation"),
)

// Named is a helper for getting the named MyComponent's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(ResourceSubtype, name)
}

func init() {
	registry.RegisterResourceSubtype(ResourceSubtype, registry.ResourceSubtype{
		Reconfigurable: wrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.SummationService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterSummationServiceHandlerFromEndpoint,
			)
		},
		RegisterRPCLiteService: func(ctx context.Context, grpcServer *grpc.Server, subtypeSvc subtype.Service) error {
			grpcServer.RegisterService(
				&pb.SummationService_ServiceDesc,
				NewServer(subtypeSvc),
			)
			return nil
		},
		RPCServiceDesc: &pb.SummationService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return newClientFromConn(conn, name, logger)
		},
	})

}

// Summation defines the Go interface for the service (should match the protobuf methods.)
type Summation interface {
	Sum(ctx context.Context, nums []float64) (float64, error)
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((Summation)(nil), actual)
}

func wrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	mc, ok := r.(Summation)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := mc.(*reconfigurableSummation); ok {
		return reconfigurable, nil
	}
	return &reconfigurableSummation{actual: mc}, nil
}

var (
	_ = Summation(&reconfigurableSummation{})
	_ = resource.Reconfigurable(&reconfigurableSummation{})
)

type reconfigurableSummation struct {
	mu     sync.RWMutex
	actual Summation
}

func (g *reconfigurableSummation) ProxyFor() interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual
}

func (g *reconfigurableSummation) Reconfigure(ctx context.Context, newMyComponent resource.Reconfigurable) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	actual, ok := newMyComponent.(*reconfigurableSummation)
	if !ok {
		return utils.NewUnexpectedTypeError(g, newMyComponent)
	}
	if err := goutils.TryClose(ctx, g.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	g.actual = actual.actual
	return nil
}

func (g *reconfigurableSummation) Sum(ctx context.Context, nums []float64) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.actual.Sum(ctx, nums)
}

// subtypeServer implements the Summation RPC service from summation.proto.
type subtypeServer struct {
	pb.UnimplementedSummationServiceServer
	s subtype.Service
}

func NewServer(s subtype.Service) pb.SummationServiceServer {
	return &subtypeServer{s: s}
}

func (s *subtypeServer) getMyService(name string) (Summation, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no summation service with name (%s)", name)
	}
	g, ok := resource.(Summation)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a Summation", name)
	}
	return g, nil
}

func (s *subtypeServer) Sum(ctx context.Context, req *pb.SumRequest) (*pb.SumResponse, error) {
	g, err := s.getMyService(req.Name)
	if err != nil {
		return nil, err
	}
	resp, err := g.Sum(ctx, req.Numbers)
	if err != nil {
		return nil, err
	}
	return &pb.SumResponse{Sum: resp}, nil
}


func newClientFromConn(conn rpc.ClientConn, name string, logger golog.Logger) Summation {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewSummationServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

type serviceClient struct {
	conn   rpc.ClientConn
	client pb.SummationServiceClient
	logger golog.Logger
}

type client struct {
	*serviceClient
	name string
}

func clientFromSvcClient(sc *serviceClient, name string) Summation {
	return &client{sc, name}
}

func (c *client) Sum(ctx context.Context, nums []float64) (float64, error) {
	resp, err := c.client.Sum(ctx, &pb.SumRequest{
		Name: c.name,
		Numbers: nums,
	})
	if err != nil {
		return 0, err
	}
	return resp.Sum, nil
}
