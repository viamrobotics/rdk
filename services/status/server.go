// Package status contains a gRPC based status service server
package status

import (
	"context"

	"google.golang.org/protobuf/types/known/structpb"

	pb "go.viam.com/rdk/proto/api/service/status/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the StatusService from status.proto.
type subtypeServer struct {
	pb.UnimplementedStatusServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a status gRPC service server.
func NewServer(s subtype.Service) pb.StatusServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("status.Service", resource)
	}
	return svc, nil
}

// If resource status contains a list and it is not of type []interface, this will fail.
func (server *subtypeServer) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	resourceNames := make([]resource.Name, 0, len(req.ResourceNames))
	for _, name := range req.ResourceNames {
		resourceNames = append(resourceNames, protoutils.ResourceNameFromProto(name))
	}

	statuses, err := svc.GetStatus(ctx, resourceNames)
	if err != nil {
		return nil, err
	}

	statusesP := make([]*pb.Status, 0, len(statuses))
	for _, status := range statuses {
		encoded, err := protoutils.StructToMap(status.Status)
		if err != nil {
			return nil, err
		}
		statusP, err := structpb.NewStruct(encoded)
		if err != nil {
			return nil, err
		}
		statusesP = append(
			statusesP,
			&pb.Status{
				Name:   protoutils.ResourceNameToProto(status.Name),
				Status: statusP,
			},
		)
	}

	return &pb.GetStatusResponse{Status: statusesP}, nil
}
