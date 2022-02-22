package objectsegmentation

import (
	"bytes"
	"context"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/objectsegmentation/v1"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// subtypeServer implements the Object Segmentation Service.
type subtypeServer struct {
	pb.UnimplementedObjectSegmentationServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a object segmentation gRPC service server.
func NewServer(s subtype.Service) pb.ObjectSegmentationServiceServer {
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
	config := &vision.Parameters3D{
		MinPtsInPlane:      int(req.MinPointsInPlane),
		MinPtsInSegment:    int(req.MinPointsInSegment),
		ClusteringRadiusMm: req.ClusteringRadiusMm,
	}
	objects, err := svc.GetObjectPointClouds(ctx, req.Name, config)
	if err != nil {
		return nil, err
	}
	protoSegments, err := segmentsToProto(req.Name, objects)
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
		err := seg.ToPCD(&buf)
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
