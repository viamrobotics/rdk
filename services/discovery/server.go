package discovery

import (
	"context"

	pb "go.viam.com/rdk/proto/api/service/discovery/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
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

func (server *subtypeServer) GetCameras(ctx context.Context, req *pb.GetCamerasRequest) (
	*pb.GetCamerasResponse, error,
) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	cameras, err := svc.GetCameras(ctx)
	if err != nil {
		return nil, err
	}
	respCams := []*pb.CameraConfig{}
	for _, conf := range cameras {
		camConf := &pb.CameraConfig{
			Label:      conf.Label,
			Status:     string(conf.Status),
			Properties: []*pb.Property{},
		}

		for _, p := range conf.Properties {
			video := &pb.Video{
				Width:       int32(p.Video.Width),
				Height:      int32(p.Video.Height),
				FrameFormat: string(p.Video.FrameFormat),
			}
			property := &pb.Property{Video: video}

			camConf.Properties = append(camConf.Properties, property)
		}
		respCams = append(respCams, camConf)
	}
	return &pb.GetCamerasResponse{Cameras: respCams}, nil
}
