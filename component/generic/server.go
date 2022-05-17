// Package generic contains a gRPC based generic service subtypeServer.
package generic

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"

	pb "go.viam.com/rdk/proto/api/component/generic/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the SensorService from sensor.proto.
type subtypeServer struct {
	pb.UnimplementedGenericServiceServer
	s subtype.Service
}

// NewServer constructs an generic gRPC service subtypeServer.
func NewServer(s subtype.Service) pb.GenericServiceServer {
	return &subtypeServer{s: s}
}

// getGeneric returns the component specified, nil if not.
func (s *subtypeServer) getGeneric(name string) (Generic, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no resource with name (%s)", name)
	}
	generic, ok := resource.(Generic)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a generic component", name)
	}
	return generic, nil
}

// Do returns an aribtrary command and returns arbitrary results
func (s *subtypeServer) Do(ctx context.Context, req *pb.DoRequest) (*pb.DoResponse, error) {
	genericDevice, err := s.getGeneric(req.Name)
	if err != nil {
		return nil, err
	}
	result, err := genericDevice.Do(ctx, req.Command.AsMap())
	if err != nil {
		return nil, err
	}
	res, err := structpb.NewStruct(result)
	if err != nil {
		return nil, err
	}
	return &pb.DoResponse{Result: res}, nil
}
