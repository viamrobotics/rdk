package objectsegmentation

import (
	"bytes"
	"context"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/vision/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// subtypeServer implements the Object Segmentation Service.
type subtypeServer struct {
	pb.UnimplementedVisionServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a object segmentation gRPC service server.
func NewServer(s subtype.Service) pb.VisionServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service() (Service, error) {
	resource := server.subtypeSvc.Resource(Name.String())
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Name)
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("objectsegmentation.Service", resource)
	}
	return svc, nil
}

func (server *subtypeServer) GetSegmenters(
	ctx context.Context,
	req *pb.GetSegmentersRequest,
) (*pb.GetSegmentersResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	names, err := svc.GetSegmenters(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetSegmentersResponse{
		Segmenters: names,
	}, nil
}

func (server *subtypeServer) GetSegmenterParameters(
	ctx context.Context,
	req *pb.GetSegmenterParametersRequest,
) (*pb.GetSegmenterParametersResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	params, err := svc.GetSegmenterParameters(ctx, req.SegmenterName)
	if err != nil {
		return nil, err
	}
	typedParams := make([]*pb.TypedParameter, len(params))
	for i, p := range params {
		typedParams[i] = &pb.TypedParameter{Name: p.Name, Type: p.Type}
	}
	return &pb.GetSegmenterParametersResponse{
		Parameters: typedParams,
	}, nil
}

// GetObjectPointClouds returns an array of objects from the frame from a camera of the underlying robot. A specific MIME type
// can be requested but may not necessarily be the same one returned. Also returns a Vector3 array of the center points of each object.
func (server *subtypeServer) GetObjectPointClouds(
	ctx context.Context,
	req *pb.GetObjectPointCloudsRequest,
) (*pb.GetObjectPointCloudsResponse, error) {
	svc, err := server.service()
	if err != nil {
		return nil, err
	}
	conf := config.AttributeMap(req.Parameters.AsMap())
	objects, err := svc.GetObjectPointClouds(ctx, req.CameraName, req.SegmenterName, conf)
	if err != nil {
		return nil, err
	}
	protoSegments, err := segmentsToProto(req.CameraName, objects)
	if err != nil {
		return nil, err
	}

	return &pb.GetObjectPointCloudsResponse{
		MimeType: utils.MimeTypePCD,
		Objects:  protoSegments,
	}, nil
}

func segmentsToProto(frame string, segs []*vision.Object) ([]*commonpb.PointCloudObject, error) {
	protoSegs := make([]*commonpb.PointCloudObject, 0, len(segs))
	for _, seg := range segs {
		var buf bytes.Buffer
		err := pointcloud.ToPCD(seg, &buf, pointcloud.PCDBinary)
		if err != nil {
			return nil, err
		}
		ps := &commonpb.PointCloudObject{
			PointCloud: buf.Bytes(),
			Geometries: &commonpb.GeometriesInFrame{
				Geometries:     []*commonpb.Geometry{seg.BoundingBox.ToProtobuf()},
				ReferenceFrame: frame,
			},
		}
		protoSegs = append(protoSegs, ps)
	}
	return protoSegs, nil
}
