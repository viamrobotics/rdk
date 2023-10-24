// Package summationapi defines a simple number summing service API for demonstration purposes.
package summationapi

import (
	"context"

	"go.viam.com/utils/rpc"

	pb "go.viam.com/rdk/examples/customresources/apis/proto/api/service/summation/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// API is the full API definition.
var API = resource.APINamespace("acme").WithServiceType("summation")

// Named is a helper for getting the named Summation's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named Summation from the given Robot.
func FromRobot(r robot.Robot, name string) (Summation, error) {
	return robot.ResourceFromRobot[Summation](r, Named(name))
}

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Summation]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterSummationServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.SummationService_ServiceDesc,
		RPCClient: func(
			ctx context.Context,
			conn rpc.ClientConn,
			remoteName string,
			name resource.Name,
			logger logging.Logger,
		) (Summation, error) {
			return newClientFromConn(conn, remoteName, name, logging.FromZapCompatible(logger)), nil
		},
	})
}

// Summation defines the Go interface for the service (should match the protobuf methods.)
type Summation interface {
	resource.Resource
	Sum(ctx context.Context, nums []float64) (float64, error)
}

// serviceServer implements the Summation RPC service from summation.proto.
type serviceServer struct {
	pb.UnimplementedSummationServiceServer
	coll resource.APIResourceCollection[Summation]
}

// NewRPCServiceServer returns a new RPC server for the summation API.
func NewRPCServiceServer(coll resource.APIResourceCollection[Summation]) interface{} {
	return &serviceServer{coll: coll}
}

func (s *serviceServer) Sum(ctx context.Context, req *pb.SumRequest) (*pb.SumResponse, error) {
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

func newClientFromConn(conn rpc.ClientConn, remoteName string, name resource.Name, logger logging.Logger) Summation {
	sc := newSvcClientFromConn(conn, remoteName, name, logger)
	return clientFromSvcClient(sc, name.ShortName())
}

func newSvcClientFromConn(conn rpc.ClientConn, remoteName string, name resource.Name, logger logging.Logger) *serviceClient {
	client := pb.NewSummationServiceClient(conn)
	sc := &serviceClient{
		Named:  name.PrependRemote(remoteName).AsNamed(),
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
	logger logging.Logger
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
