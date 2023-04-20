// Package summation defines a simple number summing service API for demonstration purposes.
package summationapi

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/rdk/examples/customresources/apis/proto/api/service/summation/v1"
	"go.viam.com/rdk/robot"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var Subtype = resource.NewSubtype(
	resource.Namespace("acme"),
	resource.ResourceTypeService,
	resource.SubtypeName("summation"),
)

// Named is a helper for getting the named Summation's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot is a helper for getting the named Summation from the given Robot.
func FromRobot(r robot.Robot, name string) (Summation, error) {
	return robot.ResourceFromRobot[Summation](r, Named(name))
}

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype[Summation]{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeColl resource.SubtypeCollection[Summation]) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.SummationService_ServiceDesc,
				NewServer(subtypeColl),
				pb.RegisterSummationServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.SummationService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (Summation, error) {
			return newClientFromConn(conn, name, logger), nil
		},
	})

}

// Summation defines the Go interface for the service (should match the protobuf methods.)
type Summation interface {
	resource.Resource
	Sum(ctx context.Context, nums []float64) (float64, error)
}

// subtypeServer implements the Summation RPC service from summation.proto.
type subtypeServer struct {
	pb.UnimplementedSummationServiceServer
	coll resource.SubtypeCollection[Summation]
}

func NewServer(coll resource.SubtypeCollection[Summation]) pb.SummationServiceServer {
	return &subtypeServer{coll: coll}
}

func (s *subtypeServer) Sum(ctx context.Context, req *pb.SumRequest) (*pb.SumResponse, error) {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	resp, err := g.Sum(ctx, req.Numbers)
	if err != nil {
		return nil, err
	}
	return &pb.SumResponse{Sum: resp}, nil
}

func newClientFromConn(conn rpc.ClientConn, name resource.Name, logger golog.Logger) Summation {
	sc := newSvcClientFromConn(conn, name, logger)
	return clientFromSvcClient(sc, name.ShortNameForClient())
}

func newSvcClientFromConn(conn rpc.ClientConn, name resource.Name, logger golog.Logger) *serviceClient {
	client := pb.NewSummationServiceClient(conn)
	sc := &serviceClient{
		Named:  name.AsNamed(),
		client: client,
		logger: logger,
	}
	return sc
}

type serviceClient struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
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
		Name:    c.name,
		Numbers: nums,
	})
	if err != nil {
		return 0, err
	}
	return resp.Sum, nil
}
