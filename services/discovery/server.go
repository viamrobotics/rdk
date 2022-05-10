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

func (server *subtypeServer) Discover(ctx context.Context, req *pb.DiscoverRequest) (*pb.DiscoverResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	keys := make([]Key, 0, len(req.Keys))
	for _, key := range req.Keys {
		keys = append(keys, Key{resource.SubtypeName(key.Subtype), key.Model})
	}

	discoveries, err := svc.Discover(ctx, keys)
	if err != nil {
		return nil, err
	}

	pbDiscoveries := make([]*pb.Discovery, 0, len(discoveries))
	for _, discovery := range discoveries {
		// InterfaceToMap necessary because structpb.NewStruct only accepts []interface{} for slices and mapstructure does not do the
		// conversion necessary.
		encoded, err := protoutils.InterfaceToMap(discovery.Discovered)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to convert discovery for %q to a form acceptable to structpb.NewStruct", discovery.Key)
		}
		pbDiscovery, err := structpb.NewStruct(encoded)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to construct a structpb.Struct from discovery for %q", discovery.Key)
		}
		pbKey := &pb.Key{Subtype: discovery.Key.model, Model: discovery.Key.model}
		pbDiscoveries = append(
			pbDiscoveries,
			&pb.Discovery{
				Key:        pbKey,
				Discovered: pbDiscovery,
			},
		)
	}

	return &pb.DiscoverResponse{Discovery: pbDiscoveries}, nil
}
