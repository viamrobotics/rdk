package discovery

import (
	"context"

	"github.com/pkg/errors"
	pb "go.viam.com/rdk/proto/api/service/discovery/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"google.golang.org/protobuf/types/known/structpb"
)

// subtypeServer implements the contract from discovery.proto.
type subtypeServer struct {
	pb.UnimplementedDiscoveryServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a framesystem gRPC service server.
func NewServer(s subtype.Service) pb.DiscoveryServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("discovery.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) Discovery(ctx context.Context, req *pb.DiscoverRequest) (*pb.DiscoverResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	resourceNames := make([]resource.Name, 0, len(req.ResourceNames))
	for _, name := range req.ResourceNames {
		resourceNames = append(resourceNames, protoutils.ResourceNameFromProto(name))
	}

	discoveries, err := svc.Discover(ctx, resourceNames)
	if err != nil {
		return nil, err
	}

	discoveriesP := make([]*pb.Discovery, 0, len(discoveries))
	for _, discovery := range discoveries {
		// InterfaceToMap necessary because structpb.NewStruct only accepts []interface{} for slices and mapstructure does not do the
		// conversion necessary.
		encoded, err := protoutils.InterfaceToMap(discovery.Discovered)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to convert discovery for %q to a form acceptable to structpb.NewStruct", discovery.Name)
		}
		discoveryP, err := structpb.NewStruct(encoded)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to construct a structpb.Struct from discovery for %q", discovery.Name)
		}
		discoveriesP = append(
			discoveriesP,
			&pb.Discovery{
				Name:       protoutils.ResourceNameToProto(discovery.Name),
				Discovered: discoveryP,
			},
		)
	}

	return &pb.DiscoverResponse{Discovery: discoveriesP}, nil
}
