// Package slam implements simultaneous localization and mapping
// This is an Experimental package
package slam

import (
	"bytes"
	"context"
	"image/jpeg"

	"go.opencensus.io/trace"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/slam/v1"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
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

func (server *subtypeServer) service(serviceName string) (Service, error) {
	resource := server.subtypeSvc.Resource(serviceName)
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Named(serviceName))
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(resource)
	}
	return svc, nil
}

// GetPosition returns a poseInFrame representing the most recent robot location and takes in the slam service name
// as an input.
func (server *subtypeServer) GetPosition(ctx context.Context, req *pb.GetPositionRequest) (
	*pb.GetPositionResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::server::GetPosition")
	defer span.End()

	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}

	p, err := svc.Position(ctx, req.Name, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	return &pb.GetPositionResponse{
		Pose: referenceframe.PoseInFrameToProtobuf(p),
	}, nil
}

// GetMap returns a mimeType and a map that is either a image byte slice or PointCloudObject defined in
// common.proto. It takes in the name of the slam service, mime type, and optional parameter including
// camera position and whether the resulting image should include the current robot position.
func (server *subtypeServer) GetMap(ctx context.Context, req *pb.GetMapRequest) (
	*pb.GetMapResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::server::GetMap")
	defer span.End()

	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}

	var pInFrame *referenceframe.PoseInFrame
	if req.CameraPosition != nil {
		pInFrame = referenceframe.ProtobufToPoseInFrame(&commonpb.PoseInFrame{Pose: req.CameraPosition})
	}
	mimeType, imageData, pcData, err := svc.GetMap(ctx, req.Name, req.MimeType, pInFrame, req.IncludeRobotMarker, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	resp := &pb.GetMapResponse{}
	switch mimeType {
	case utils.MimeTypeJPEG:
		_, spanEncode := trace.StartSpan(ctx, "slam::server::GetMap:Encode")
		defer spanEncode.End()

		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, imageData, nil); err != nil {
			return nil, err
		}

		mapData := &pb.GetMapResponse_Image{Image: buf.Bytes()}
		resp = &pb.GetMapResponse{
			MimeType: mimeType,
			Map:      mapData,
		}
	case utils.MimeTypePCD:
		_, spanToPCD := trace.StartSpan(ctx, "slam::server::GetMap:ToPCD")
		defer spanToPCD.End()

		var buf bytes.Buffer
		if err := pointcloud.ToPCD(pcData.PointCloud, &buf, pointcloud.PCDBinary); err != nil {
			return nil, err
		}
		mapData := &pb.GetMapResponse_PointCloud{PointCloud: &commonpb.PointCloudObject{PointCloud: buf.Bytes()}}
		resp = &pb.GetMapResponse{
			MimeType: mimeType,
			Map:      mapData,
		}
	}

	return resp, nil
}
