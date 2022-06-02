package slam

import (
	"context"

	pb "go.viam.com/rdk/proto/api/service/slam/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the SLAMService from the slam proto.
type subtypeServer struct {
	pb.UnimplementedSLAMServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a the slam gRPC service server.
func NewServer(s subtype.Service) pb.SLAMServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	name := Name
	resource := server.subtypeSvc.Resource(name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("slam.Service", resource)
	}
	return svc, nil
}

// GetPosition returns a poseInFrame from the slam library being run and takes in a slam service name as an input.
func (server *subtypeServer) GetPosition(ctx context.Context, req *pb.GetPositionRequest) (
	*pb.GetPositionResponse, error,
) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}

	p, err := svc.GetPosition(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.GetPositionResponse{
		Pose: p,
	}, nil
}

// GetMap returns a mimeType and a map that is either a image byte slice or pointCloudObject defined in
// common.proto. It takes in the name of slam service as well as a mime type, and optional parameters
// including camera position parameter and if the resulting image should include the current robot position.
func (server *subtypeServer) GetMap(ctx context.Context, req *pb.GetMapRequest) (
	*pb.GetMapResponse, error,
) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}

	mimeType, imageData, pcData, err := svc.GetMap(ctx, req.Name, req.MimeType, req.CameraPosition, req.IncludeRobotMarker)
	if err != nil {
		return nil, err
	}

	resp := &pb.GetMapResponse{}
	switch mimeType {
	case utils.MimeTypeJPEG:
		mapData := &pb.GetMapResponse_Image{Image: imageData}
		resp = &pb.GetMapResponse{
			MimeType: mimeType,
			Map:      mapData,
		}
	case utils.MimeTypePCD:
		mapData := &pb.GetMapResponse_PointCloud{PointCloud: pcData}
		resp = &pb.GetMapResponse{
			MimeType: mimeType,
			Map:      mapData,
		}
	}

	return resp, nil
}
