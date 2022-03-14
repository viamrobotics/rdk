// Package status contains a gRPC based status service server
package status

import (
	"context"
	"time"

	"github.com/pkg/errors"
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
		// InterfaceToMap necessary because structpb.NewStruct only accepts []interface{} for slices and mapstructure does not do the
		// conversion necessary.
		encoded, err := protoutils.InterfaceToMap(status.Status)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to convert status for %q to a form acceptable to structpb.NewStruct", status.Name)
		}
		statusP, err := structpb.NewStruct(encoded)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to construct a structpb.Struct from status for %q", status.Name)
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

const defaultStreamInterval = 1 * time.Second

func (server *subtypeServer) StreamStatus(req *pb.StreamStatusRequest, streamServer pb.StatusService_StreamStatusServer) error {
	every := defaultStreamInterval
	if reqEvery := req.Every.AsDuration(); reqEvery != time.Duration(0) {
		every = reqEvery
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-streamServer.Context().Done():
			return streamServer.Context().Err()
		default:
		}
		select {
		case <-streamServer.Context().Done():
			return streamServer.Context().Err()
		case <-ticker.C:
		}
		status, err := server.GetStatus(streamServer.Context(), &pb.GetStatusRequest{ResourceNames: req.ResourceNames})
		if err != nil {
			return err
		}
		if err := streamServer.Send(&pb.StreamStatusResponse{Status: status.Status}); err != nil {
			return err
		}
	}
}
